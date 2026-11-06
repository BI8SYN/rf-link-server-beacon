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

func TestSASCommandsOnlyShowSelectedDeployment(t *testing.T) {
	for _, mode := range []string{"native", "compose", "docker"} {
		t.Run(mode, func(t *testing.T) {
			r, service := newStationRig(t)
			defer service.Close()
			r.mu.Lock()
			r.info.ServerTime = uint64(time.Now().Unix())
			r.mu.Unlock()
			text, err := SASCommands(context.Background(), r.cfg, mode, SASSetupOptions{Scenario: "convert"})
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
	if _, err := SASCommands(context.Background(), config.Config{}, "unknown", SASSetupOptions{}); err == nil || !strings.Contains(err.Error(), "deployment") {
		t.Fatal("deployment not validated first", err)
	}
}

func TestSASScenariosRequireIntentAndSeparatePorts(t *testing.T) {
	for _, tc := range []struct {
		o    SASSetupOptions
		mode string
		port uint64
	}{
		{SASSetupOptions{}, "native", 1883},
		{SASSetupOptions{Scenario: "bad"}, "native", 1883},
		{SASSetupOptions{Scenario: "coexist"}, "compose", 1883},
		{SASSetupOptions{Scenario: "convert"}, "full-stack", 1883},
		{SASSetupOptions{Scenario: "coexist", MQTTNetwork: "bad;name"}, "compose", 1884},
		{SASSetupOptions{Scenario: "coexist", AuthURL: "http://user:secret@host/auth"}, "compose", 1884},
	} {
		if err := ValidateSASSetup(tc.o, tc.mode, tc.port); err == nil {
			t.Fatalf("unsafe setup accepted: %+v", tc)
		}
	}
	if err := ValidateSASSetup(SASSetupOptions{Scenario: "coexist", FMOPort: 1885}, "compose", 1884); err != nil {
		t.Fatal(err)
	}
}
func TestCoexistGeneratesSeparateSASAndListener(t *testing.T) {
	for _, mode := range []string{"compose", "native", "docker"} {
		t.Run(mode, func(t *testing.T) {
			r, s := newStationRig(t)
			defer s.Close()
			r.mu.Lock()
			r.info.ServerTime = uint64(time.Now().Unix())
			r.mu.Unlock()
			r.cfg.Station.Port = 1884
			before, _ := os.ReadFile(filepath.Join(s.cfg.StateDir, "state.json"))
			text, err := SASCommands(context.Background(), r.cfg, mode, SASSetupOptions{Scenario: "coexist"})
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"1884", "listeners.tcp.rf_link", "18081/auth", r.info.Fingerprint} {
				if !strings.Contains(text, want) {
					t.Fatal("missing", want, text)
				}
			}
			for _, bad := range []string{"更新原SAS", "合并到原SAS", "重建原SAS"} {
				if strings.Contains(text, bad) {
					t.Fatal("coexist would replace FMO", text)
				}
			}
			if mode == "native" && (!strings.Contains(text, ".rf-link-sas/config.json") || !strings.Contains(text, "--http-port 18081")) {
				t.Fatal("native state is not separate", text)
			}
			if mode != "native" && !strings.Contains(text, "compose.sas.yaml") {
				t.Fatal(text)
			}
			after, _ := os.ReadFile(filepath.Join(s.cfg.StateDir, "state.json"))
			if string(before) != string(after) {
				t.Fatal("state changed")
			}
		})
	}
}
func TestCoexistContainerBrokerUsesItsNetwork(t *testing.T) {
	r, s := newStationRig(t)
	defer s.Close()
	r.mu.Lock()
	r.info.ServerTime = uint64(time.Now().Unix())
	r.mu.Unlock()
	r.cfg.Station.Port = 1884
	text, err := SASCommands(context.Background(), r.cfg, "compose", SASSetupOptions{Scenario: "coexist", MQTTNetwork: "fmo_default"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "http://rf-link-sas:8080/auth") || !strings.Contains(text, "RF_LINK_MQTT_NETWORK=fmo_default") || !strings.Contains(text, "compose.sas-network.yaml") {
		t.Fatal(text)
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
func TestConvertExplicitlyDisclosesIdentityReplacement(t *testing.T) {
	r, s := newStationRig(t)
	defer s.Close()
	r.mu.Lock()
	r.info.ServerTime = uint64(time.Now().Unix())
	r.mu.Unlock()
	text, err := SASCommands(context.Background(), r.cfg, "compose", SASSetupOptions{Scenario: "convert"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "先备份") || !strings.Contains(text, "不再保留原FMO身份") || strings.Contains(text, "listeners.tcp.rf_link") {
		t.Fatal(text)
	}
}
