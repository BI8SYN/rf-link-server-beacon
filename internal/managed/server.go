package managed

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/BI8SYN/rf-link-server-beacon/internal/app"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
)

const (
	dataDir      = "/data"
	exchangeDir  = "/exchange"
	passwordFile = "/dashboard/password"
)

func configFromEnv() (config.Simple, error) {
	cfg := config.Simple{
		Callsign: os.Getenv("BEACON_CALLSIGN"), Grid: os.Getenv("BEACON_GRID"),
		Server: config.Server{Name: os.Getenv("BEACON_NAME"), Host: os.Getenv("BEACON_MQTT_HOST"), Port: 1883},
		SSID:   15, FilterKM: 5000, CountryCode: os.Getenv("BEACON_COUNTRY"),
		Applicant: &config.Applicant{Name: os.Getenv("BEACON_OWNER"), Email: os.Getenv("BEACON_EMAIL"), AuthorizationNote: config.DefaultAuthorizationNote},
	}
	for _, field := range []struct {
		key    string
		target *uint64
	}{{"BEACON_SSID", &cfg.SSID}, {"BEACON_MQTT_PORT", &cfg.Server.Port}} {
		if v := os.Getenv(field.key); v != "" {
			n, e := strconv.ParseUint(v, 10, 16)
			if e != nil {
				return cfg, fmt.Errorf("invalid %s", field.key)
			}
			*field.target = n
		}
	}
	if cfg.Grid == "" {
		return cfg, errors.New("BEACON_GRID is required")
	}
	if cfg.Callsign == "" {
		return cfg, errors.New("BEACON_CALLSIGN is required")
	}
	if cfg.Server.Name == "" {
		return cfg, errors.New("BEACON_NAME is required")
	}
	if cfg.Server.Host == "" {
		return cfg, errors.New("BEACON_MQTT_HOST is required")
	}
	if cfg.Applicant.Name == "" {
		return cfg, errors.New("BEACON_OWNER is required")
	}
	if cfg.Applicant.Email == "" {
		return cfg, errors.New("BEACON_EMAIL is required")
	}
	return cfg, cfg.Applicant.Validate()
}

func atomicWrite(path string, raw []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	f, e := os.CreateTemp(dir, ".managed-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if path == passwordFile {
		if e = f.Chown(-1, 1000); e != nil {
			return e
		}
	}
	if e = f.Chmod(mode); e != nil {
		return e
	}
	if _, e = f.Write(raw); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Rename(f.Name(), path); e != nil {
		return e
	}
	d, e := os.Open(dir)
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}

type stackSAS struct {
	gate *gate
	path string
	raw  []byte
}

func (s *stackSAS) Configure(raw []byte) error {
	if !bytes.Equal(raw, s.raw) {
		s.Disable()
		if e := atomicWrite(s.path, raw, 0644); e != nil {
			return e
		}
		s.raw = raw
	}
	digest := sha256.Sum256(raw)
	s.gate.setIdentity(hex.EncodeToString(digest[:]))
	return nil
}
func (s *stackSAS) Ready() bool { return s.gate.allowed() }
func (s *stackSAS) Disable()    { s.gate.setIdentity("") }

func Status(out io.Writer) error {
	raw, e := os.ReadFile(filepath.Join(dataDir, "progress.json"))
	if errors.Is(e, os.ErrNotExist) {
		_, e = fmt.Fprintln(out, `{"phase":"starting"}`)
		return e
	}
	if e != nil {
		return e
	}
	_, e = out.Write(raw)
	return e
}

func Password(out io.Writer) error {
	raw, e := os.ReadFile(passwordFile)
	if e != nil {
		return e
	}
	_, e = out.Write(raw)
	return e
}

func Run(ctx context.Context, out io.Writer) (err error) {
	progressPath := filepath.Join(dataDir, "progress.json")
	var last app.Progress
	report := func(p app.Progress) error {
		if p.UpdatedAt == 0 {
			p.UpdatedAt = time.Now().Unix()
		}
		raw, e := json.Marshal(p)
		if e != nil {
			return e
		}
		if e = atomicWrite(progressPath, append(raw, '\n'), 0600); e != nil {
			return e
		}
		if p.Phase != last.Phase || p.LastPublished != last.LastPublished || p.Reason != last.Reason || p.EnrollmentID != last.EnrollmentID {
			fmt.Fprintln(out, string(raw))
		}
		last = p
		return nil
	}
	defer func() {
		if err != nil && !errors.Is(err, context.Canceled) {
			last.Phase = "failed"
			last.Reason = err.Error()
			last.RetryAt = 0
			last.UpdatedAt = 0
			_ = report(last)
		}
	}()
	cfgPath := filepath.Join(dataDir, "config.json")
	if _, e := os.Stat(cfgPath); errors.Is(e, os.ErrNotExist) {
		simple, e := configFromEnv()
		if e != nil {
			return e
		}
		if e = config.Initialize(dataDir, simple); e != nil {
			return e
		}
	} else if e != nil {
		return e
	}
	cfg, e := config.Load(cfgPath)
	if e != nil {
		return e
	}

	if cfg.Signer.BaseURL == config.DefaultOrigin {
		simple, e := configFromEnv()
		if e != nil {
			return e
		}
		if e = config.ApplyManagedSettings(cfgPath, simple); e != nil {
			return e
		}
		cfg, e = config.Load(cfgPath)
		if e != nil {
			return e
		}
	}
	svc, e := app.NewService(cfg)
	if e != nil {
		return e
	}
	defer svc.Close()

	if _, e = os.Stat(passwordFile); errors.Is(e, os.ErrNotExist) {
		var secret [24]byte
		if _, e = rand.Read(secret[:]); e != nil {
			return e
		}
		if e = atomicWrite(passwordFile, []byte(hex.EncodeToString(secret[:])+"\n"), 0640); e != nil {
			return e
		}
	} else if e != nil {
		return e
	}

	g := newGate("emqx:1883", "http://sas:8080", "/brokerdata/beacon-boot", "/sasdata/beacon-loaded")
	mqtt, e := net.Listen("tcp", ":1883")
	if e != nil {
		return e
	}
	defer mqtt.Close()
	auth, e := net.Listen("tcp", ":8081")
	if e != nil {
		return e
	}
	defer auth.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	httpd := &http.Server{Handler: g, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, MaxHeaderBytes: 8192}
	defer httpd.Close()
	go func() {
		if e := httpd.Serve(auth); e != nil && !errors.Is(e, http.ErrServerClosed) {
			cancel()
		}
	}()
	go func() {
		if g.relay(ctx, mqtt) != nil {
			cancel()
		}
	}()
	go func() {
		for ctx.Err() == nil {
			_ = g.check(ctx)
			if app.Wait(ctx, 2*time.Second) != nil {
				return
			}
		}
	}()
	return svc.Manage(ctx, &stackSAS{gate: g, path: filepath.Join(exchangeDir, "sas.json")}, report)
}
