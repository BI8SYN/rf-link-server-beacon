package config

import (
	"encoding/json"
	"errors"
	"github.com/BI8SYN/rf-link-server-beacon/internal/jsonx"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Applicant            *Applicant `json:"applicant,omitempty"`
	Schema               uint64     `json:"schema_version"`
	CoordinatesConfirmed bool       `json:"coordinates_confirmed"`
	Callsign             string     `json:"callsign"`
	Label                string     `json:"label"`
	PurposeNote          string     `json:"purpose_note"`
	Signer               struct {
		BaseURL           string `json:"base_url"`
		CredentialFile    string `json:"credential_file"`
		AllowLoopbackHTTP bool   `json:"allow_loopback_http"`
	} `json:"signer"`
	Trust struct {
		Roots      []string `json:"root_fingerprints"`
		CRLURLs    []string `json:"crl_urls,omitempty"`
		CRLOrigins []string `json:"crl_origins,omitempty"`
	} `json:"trust"`
	StateDir   string           `json:"state_dir"`
	Station    protocol.Station `json:"station"`
	Statistics struct {
		Online uint64 `json:"online"`
		Peak   uint64 `json:"peak"`
	} `json:"statistics"`
	APRS struct {
		Host         string `json:"host"`
		Port         uint64 `json:"port"`
		PasscodeFile string `json:"passcode_file"`
		Interval     uint64 `json:"interval_seconds"`
	} `json:"aprs"`
}

func Load(path string) (Config, error) {
	var c Config
	f, e := os.Open(path)
	if e != nil {
		return c, errors.New("cannot open configuration")
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || info.Size() > 65536 {
		return c, errors.New("configuration too large")
	}
	raw, e := io.ReadAll(io.LimitReader(f, 65537))
	if e != nil || len(raw) > 65536 {
		return c, errors.New("cannot read configuration")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) == nil {
		if _, ok := fields["server"]; ok {
			return loadSimple(raw, path)
		}
	}
	if e = jsonx.Decode(raw, &c); e != nil {
		return c, errors.New("invalid configuration JSON or fields")
	}
	return c, c.Validate()
}
func (c Config) Validate() error {
	if c.Schema != 1 || !c.CoordinatesConfirmed || !protocol.ValidCallsign(c.Callsign) || c.Callsign == "YOURCALL" || c.Callsign == "N0CALL" || !protocol.ValidText(c.Label, 128) || !protocol.ValidText(c.PurposeNote, 1024) || c.Station.Validate() != nil || strings.HasSuffix(c.Station.Host, ".invalid") || c.Statistics.Online > c.Statistics.Peak || c.Statistics.Peak > protocol.MaxInteger || !protocol.ValidHost(c.APRS.Host) || c.APRS.Port == 0 || c.APRS.Port > 65535 || c.APRS.Interval != 600 || !filepath.IsAbs(c.StateDir) || !filepath.IsAbs(c.Signer.CredentialFile) || (c.APRS.PasscodeFile != "" && !filepath.IsAbs(c.APRS.PasscodeFile)) {
		return errors.New("invalid configuration; replace placeholders and confirm coordinates")
	}
	if e := URL(c.Signer.BaseURL, c.Signer.AllowLoopbackHTTP); e != nil {
		return e
	}
	u, _ := url.Parse(c.Signer.BaseURL)
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(u.Hostname(), ".invalid") {
		return errors.New("signer base URL must be an explicit origin")
	}
	if len(c.Trust.Roots) == 0 || (len(c.Trust.CRLURLs) < 2 && len(c.Trust.CRLOrigins) == 0) {
		return errors.New("trusted root fingerprints and explicit CRL URLs required")
	}
	for _, pin := range c.Trust.Roots {
		if _, e := protocol.Decode64(pin, 32); e != nil {
			return errors.New("invalid trusted root fingerprint")
		}
	}
	for _, origin := range c.Trust.CRLOrigins {
		if e := URL(origin, c.Signer.AllowLoopbackHTTP); e != nil {
			return e
		}
		expected, e := Origin(origin)
		if e != nil || expected != origin {
			return errors.New("CRL allowlist requires exact origins")
		}
	}
	for _, source := range c.Trust.CRLURLs {
		if e := URL(source, c.Signer.AllowLoopbackHTTP); e != nil {
			return e
		}
	}
	return nil
}
func URL(raw string, loopback bool) error {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return errors.New("invalid service URL")
	}
	if u.Scheme == "https" {
		return nil
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme == "http" && loopback && (u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())) {
		return nil
	}
	return errors.New("HTTPS required; HTTP is permitted only for explicitly enabled loopback testing")
}
func Secret(path string) (string, error) {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 4096 {
		return "", errors.New("secret file must be a private regular file (0600)")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return "", errors.New("cannot read secret file")
	}
	s := strings.TrimRight(string(b), "\r\n")
	if s == "" || strings.ContainsAny(s, "\r\n\x00 \t") {
		return "", errors.New("secret must contain one nonempty token")
	}
	return s, nil
}
