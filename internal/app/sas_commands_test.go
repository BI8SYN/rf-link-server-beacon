package app

import (
	"context"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSASCommandsAreVerifiedReadOnlyAndWorkWhileDaemonHoldsLock(t *testing.T) {
	r, s := newStationRig(t)
	defer s.Close()
	r.mu.Lock()
	r.info.ServerTime = uint64(time.Now().Unix())
	r.mu.Unlock()
	before, e := os.ReadFile(filepath.Join(s.cfg.StateDir, "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	text, e := SASCommands(context.Background(), r.cfg, "full-stack")
	if e != nil {
		t.Fatal("command generation cannot run next to live daemon", e)
	}
	for _, want := range []string{"SAS_SERVER_UID=20", "SAS_SERVER_CALLSIGN=BI8SYN", "SAS_MQTT_HOST=voice.example.com", "SAS_MQTT_PORT=1883", "SAS_CERT_FINGERPRINT=" + r.info.Fingerprint, "SAS_ISSUER_SN="} {
		if !strings.Contains(text, want) {
			t.Fatal("missing verified SAS parameter", want)
		}
	}
	if strings.Contains(text, "./sas --") || strings.Contains(text, "command:") {
		t.Fatal("full-stack output includes other deployments")
	}

	after, _ := os.ReadFile(filepath.Join(s.cfg.StateDir, "state.json"))
	if string(before) != string(after) {
		t.Fatal("read-only guide changed runtime state")
	}
	if strings.Contains(text, localToken) {
		t.Fatal("machine credential leaked into setup commands")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) != 0 {
		t.Fatal("command generation requested a STATION signature")
	}
}

func TestSASCommandsDoesNotCreateCredentialsForPendingIdentity(t *testing.T) {
	r, s := newStationRig(t)
	defer s.Close()
	s.state.Credential = nil
	if e := s.store.Write(s.state); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(filepath.Join(s.cfg.StateDir, "state.json"))
	if _, e := SASCommands(context.Background(), r.cfg, "full-stack"); e == nil {
		t.Fatal("pending identity produced setup commands")
	}
	after, _ := os.ReadFile(filepath.Join(s.cfg.StateDir, "state.json"))
	if string(before) != string(after) {
		t.Fatal("SAS command query attempted to mint credentials")
	}
}

func TestSASCommandsOnlyShowSelectedDeployment(t *testing.T) {
	for _, mode := range []string{"native", "compose", "docker"} {
		t.Run(mode, func(t *testing.T) {
			r, service := newStationRig(t)
			defer service.Close()
			r.mu.Lock()
			r.info.ServerTime = uint64(time.Now().Unix())
			r.mu.Unlock()
			text, err := SASCommands(context.Background(), r.cfg, mode)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(text, "SAS_SERVER_UID=") {
				t.Fatal("unrelated env instructions")
			}
			if !strings.Contains(text, "20") || !strings.Contains(text, r.info.Fingerprint) {
				t.Fatal("missing identity")
			}
			switch mode {
			case "native":
				if !strings.Contains(text, "./sas --server-uid 20") || strings.Contains(text, "services:") {
					t.Fatal(text)
				}
			case "compose":
				if !strings.Contains(text, "command:") || strings.Contains(text, "./sas --") {
					t.Fatal(text)
				}
			case "docker":
				if !strings.Contains(text, "--server-uid 20") || strings.Contains(text, "services:") || strings.Contains(text, "./sas --") {
					t.Fatal(text)
				}
			}
		})
	}
}
func TestSASCommandsRejectUnknownDeploymentBeforeNetwork(t *testing.T) {
	if _, err := SASCommands(context.Background(), config.Config{}, "unknown"); err == nil || !strings.Contains(err.Error(), "deployment") {
		t.Fatal("deployment not validated first", err)
	}
}
