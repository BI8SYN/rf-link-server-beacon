package app

import (
	"context"
	"encoding/json"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplicationPersistsReceiptBeforeFirstRequestAndRecoversLostReply(t *testing.T) {
	dir := t.TempDir()
	var firstBody, firstAccess string
	attempt := 0
	const serverID = "550e8400-e29b-41d4-a716-446655440000"
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		raw, _ := os.ReadFile(filepath.Join(dir, "state", "state.json"))
		var disk State
		if json.Unmarshal(raw, &disk) != nil || disk.ApplicationAccessToken == "" || disk.Enrollment == nil {
			t.Error("request preceded durable receipt")
		}
		if r.Header.Get("Authorization") != "Bearer "+disk.ApplicationAccessToken {
			t.Error("wrong receipt authorization")
		}
		encoded, _ := json.Marshal(disk.Enrollment)
		body, _ := io.ReadAll(r.Body)
		if string(body) != string(encoded) || disk.Enrollment.Station != nil {
			t.Error("new application must send exactly the durable identity-only body")
		}
		if firstBody == "" {
			firstBody = string(encoded)
			firstAccess = disk.ApplicationAccessToken
		} else if firstBody != string(encoded) || firstAccess != disk.ApplicationAccessToken {
			t.Error("restart changed application intent or secret")
		}
		if attempt == 1 {
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(signer.Created{ServerID: serverID, EnrollmentID: disk.Enrollment.RequestID, BroadcastID: serverID, State: "pending_review"})
	}))
	defer httpServer.Close()
	cfg := applicationConfig(t, dir, httpServer.URL)
	svc, e := NewService(cfg)
	if e != nil {
		t.Fatal("service still requires a preconfigured machine token", e)
	}
	if _, e = svc.Enroll(context.Background()); e == nil {
		t.Fatal("lost reply accepted")
	}
	svc.Close()
	svc, e = NewService(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer svc.Close()
	if _, e = svc.Enroll(context.Background()); e != nil {
		t.Fatal(e)
	}
	if attempt != 2 {
		t.Fatal("unexpected automatic duplicate submission")
	}
	if _, e = os.Stat(cfg.Signer.CredentialFile); !os.IsNotExist(e) {
		t.Fatal("pending application created a manual machine token file")
	}
	info, _ := os.Stat(filepath.Join(cfg.StateDir, "state.json"))
	if info.Mode().Perm() != 0600 || !strings.HasPrefix(firstAccess, "fmba1.") {
		t.Fatal("receipt storage not private")
	}
}

func TestLegacyApplicationLostReplyReplaysOriginalStationWithoutReenrollment(t *testing.T) {
	dir := t.TempDir()
	var original string
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		raw, _ := io.ReadAll(r.Body)
		if string(raw) != original || r.Header.Get("Authorization") != "Bearer fmba1."+localID+".AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" {
			t.Error("legacy application recovery rewrote original intent or receipt")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(signer.Created{ServerID: localID, EnrollmentID: localID, BroadcastID: localID, State: "pending_review"})
	}))
	defer server.Close()
	cfg := applicationConfig(t, dir, server.URL)
	s, e := NewService(cfg)
	if e != nil {
		t.Fatal(e)
	}
	station := cfg.Station
	s.state.Enrollment = &signer.Enrollment{RequestID: localID, Callsign: cfg.Callsign, Label: cfg.Label, PurposeNote: cfg.PurposeNote, Applicant: cfg.Applicant, Station: &station}
	s.state.ApplicationAccessToken = "fmba1." + localID + ".AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	raw, _ := json.Marshal(s.state.Enrollment)
	original = string(raw)
	if e = s.store.Write(s.state); e != nil {
		t.Fatal(e)
	}
	s.Close()
	cfg.Station.Host = "changed.example.com"
	cfg.Station.Name = "Changed locally"
	s, e = NewService(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.Enroll(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Enroll(context.Background()); e != nil {
		t.Fatal(e)
	}
	if requests != 1 || s.state.Enrollment.Station.Host != station.Host || s.state.EnrollmentID != localID {
		t.Fatal("cutover recreated or changed old enrollment")
	}
}
func applicationConfig(t *testing.T, dir, origin string) config.Config {
	t.Helper()
	simple := config.Simple{Callsign: "AB1CD", SSID: 15, Grid: "FN31pr", Server: config.Server{Name: "Application Test", Host: "voice.example.com", Port: 1883}, CountryCode: "US", FilterKM: 5000, ServiceURL: origin, Applicant: &config.Applicant{Name: "Test Owner", Email: "owner@example.com", AuthorizationNote: "Synthetic test data; not proof of actual ownership"}}
	if e := config.Initialize(dir, simple); e != nil {
		t.Fatal(e)
	}
	cfg, e := config.Load(filepath.Join(dir, "config.json"))
	if e != nil {
		t.Fatal(e)
	}
	return cfg
}
