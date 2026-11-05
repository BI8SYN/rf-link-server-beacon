package app

import (
	"context"
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
	text, e := SASCommands(context.Background(), r.cfg)
	if e != nil {
		t.Fatal("command generation cannot run next to live daemon", e)
	}
	for _, want := range []string{"--server-uid 20", "--server-callsign BI8SYN", "--mqtt-host voice.example.com", "--mqtt-port 1883", "--cert-fingerprint " + r.info.Fingerprint, "--http-addr 127.0.0.1", "SAS_SERVER_UID=20", "SAS_MQTT_HOST=voice.example.com", "SAS_ISSUER_SN=", "http://sas:8080/auth", "0.0.0.0"} {
		if !strings.Contains(text, want) {
			t.Fatal("missing verified SAS parameter", want)
		}
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
	if _, e := SASCommands(context.Background(), r.cfg); e == nil {
		t.Fatal("pending identity produced setup commands")
	}
	after, _ := os.ReadFile(filepath.Join(s.cfg.StateDir, "state.json"))
	if string(before) != string(after) {
		t.Fatal("SAS command query attempted to mint credentials")
	}
}
