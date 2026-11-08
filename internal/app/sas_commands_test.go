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
	text, e := SASCommands(context.Background(), r.cfg, "full-stack", SASSetupOptions{Scenario: "new-server"})
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
	if _, e := SASCommands(context.Background(), r.cfg, "full-stack", SASSetupOptions{Scenario: "new-server"}); e == nil {
		t.Fatal("pending identity produced setup commands")
	}
	after, _ := os.ReadFile(filepath.Join(s.cfg.StateDir, "state.json"))
	if string(before) != string(after) {
		t.Fatal("SAS command query attempted to mint credentials")
	}
}

func TestSASCommandsRejectUnknownDeploymentBeforeNetwork(t *testing.T) {
	if _, err := SASCommands(context.Background(), config.Config{}, "unknown", SASSetupOptions{}); err == nil || !strings.Contains(err.Error(), "deployment") {
		t.Fatal("deployment not validated first", err)
	}
}

func TestSASTwoScenarios(t *testing.T) {
	for _, scenario := range []string{"", "bad", "convert"} {
		if err := ValidateSASSetup(SASSetupOptions{Scenario: scenario}, "native", 1883); err == nil {
			t.Fatal("retired scenario accepted", scenario)
		}
	}
	if err := ValidateSASSetup(SASSetupOptions{Scenario: "coexist"}, "full-stack", 1883); err == nil {
		t.Fatal("coexist creates broker")
	}
	for _, mode := range []string{"native", "compose", "docker"} {
		if err := ValidateSASSetup(SASSetupOptions{Scenario: "coexist"}, mode, 1883); err != nil {
			t.Fatal("shared port rejected", err)
		}
	}
}
func TestCoexistAddsIdentityToExistingSAS(t *testing.T) {
	for _, mode := range []string{"native", "compose", "docker"} {
		t.Run(mode, func(t *testing.T) {
			r, s := newStationRig(t)
			defer s.Close()
			r.mu.Lock()
			r.info.ServerTime = uint64(time.Now().Unix())
			r.mu.Unlock()
			before, _ := os.ReadFile(filepath.Join(s.cfg.StateDir, "state.json"))
			text, err := SASCommands(context.Background(), r.cfg, mode, SASSetupOptions{Scenario: "coexist"})
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"tools/sas-config.py", `"uid":20`, `"callsign":"BI8SYN"`, `"mqtt_host":"voice.example.com"`, `"mqtt_port":1883`, `"certFingerprint":"` + r.info.Fingerprint + `"`, "Mqtt.Clusters", "AllowIssuerSn", "保留原"} {
				if !strings.Contains(text, want) {
					t.Fatal("missing", want, text)
				}
			}
			for _, bad := range []string{"--server-uid", "listeners.tcp", "compose.sas", "1884", "18081", localToken} {
				if strings.Contains(text, bad) {
					t.Fatal("unsafe/unrelated output", bad, text)
				}
			}
			switch mode {
			case "native":
				if !strings.Contains(text, "systemctl restart") || strings.Contains(text, "docker cp") {
					t.Fatal(text)
				}
			case "compose":
				if !strings.Contains(text, "docker compose restart") || strings.Contains(text, "systemctl restart") {
					t.Fatal(text)
				}
			case "docker":
				if !strings.Contains(text, "docker restart") || strings.Contains(text, "docker compose restart") {
					t.Fatal(text)
				}
			}
			after, _ := os.ReadFile(filepath.Join(s.cfg.StateDir, "state.json"))
			if string(before) != string(after) {
				t.Fatal("runtime state changed")
			}
			r.mu.Lock()
			defer r.mu.Unlock()
			if len(r.requests) != 0 {
				t.Fatal("requested signature")
			}
		})
	}
}
func TestNewServerNeverUsesExistingNativeConfig(t *testing.T) {
	r, s := newStationRig(t)
	defer s.Close()
	r.mu.Lock()
	r.info.ServerTime = uint64(time.Now().Unix())
	r.mu.Unlock()
	text, err := SASCommands(context.Background(), r.cfg, "native", SASSetupOptions{Scenario: "new-server"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, `--config "$HOME/.rf-link-sas/config.json"`) || !strings.Contains(text, `--roots-dir "$HOME/.rf-link-sas/roots"`) {
		t.Fatal(text)
	}
}
