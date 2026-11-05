package app

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"github.com/BI8SYN/rf-link-server-beacon/internal/jsonx"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"github.com/BI8SYN/rf-link-server-beacon/internal/testutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoopbackEnrollmentToSignedAPRSAndRestartThrottle(t *testing.T) {
	now := time.Now()
	if now.Unix()%600 >= 590 {
		now = now.Add(20 * time.Second)
	}
	n := uint64(now.Unix())
	const id = "95b976da-d670-44ec-bb72-b876ad05f134"
	const bid = "47fe8ebd-646c-42d3-b0e9-b6f402fcb141"
	var enrollmentID string
	var serverID string = id
	var info signer.Identity
	var f testutil.Fixture
	signatures := 0
	failOnce := true
	var pendingID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		switch {
		case r.URL.Path == "/root":
			if r.Header.Get("Authorization") != "" {
				t.Error("token leaked to public PKI")
			}
			enc.Encode(f.RootCRL)
		case r.URL.Path == "/inter":
			enc.Encode(f.IntermediateCRL)
		case r.URL.Path == "/api/v1/server-broadcast/enrollments" && r.Method == "POST":
			var req signer.Enrollment
			json.NewDecoder(r.Body).Decode(&req)
			enrollmentID = req.RequestID
			info.EnrollmentID = enrollmentID
			w.WriteHeader(201)
			enc.Encode(signer.Created{ServerID: serverID, EnrollmentID: enrollmentID, BroadcastID: bid, State: "pending_review"})
		case strings.HasSuffix(r.URL.Path, "/credentials"):
			var req struct {
				RequestID string `json:"requestId"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			enc.Encode(signer.Credential{RequestID: req.RequestID, EnrollmentID: enrollmentID, BroadcastID: bid, ServerID: serverID, CredentialID: id, MachineToken: "fmb1." + id + ".AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", ExpiresAt: n + 60000, RetryUntil: n + 900})
		case strings.HasPrefix(r.URL.Path, "/api/v1/server-broadcast/enrollments/"):
			fp := info.Fingerprint
			enc.Encode(signer.Status{ServerID: serverID, EnrollmentID: enrollmentID, BroadcastID: bid, State: "ready", Reason: "", ReasonCode: "", Fingerprint: &fp, PollAfter: 600})
		case r.URL.Path == "/api/v1/server-broadcast/identities/"+bid:
			enc.Encode(info)
		case strings.HasSuffix(r.URL.Path, "/station-signatures"):
			var req signer.SignRequest
			json.NewDecoder(r.Body).Decode(&req)
			if pendingID != "" && pendingID != req.RequestID {
				t.Error("POST retry changed ID")
			}
			pendingID = req.RequestID
			if failOnce {
				failOnce = false
				w.WriteHeader(503)
				fmt.Fprint(w, `{"error":{"code":"signing_unavailable","message":"try later"},"correlationId":"test"}`)
				return
			}
			signatures++
			cert := f.Bundle.User.Cert()
			sig := protocol.B64(ed25519.Sign(f.UserKey, protocol.StationTBS(cert, req.Station, req.Online, req.Peak, n/600)))
			line, _ := protocol.Line(cert, req.Station, req.Online, req.Peak, sig)
			enc.Encode(signer.Signature{RequestID: req.RequestID, BroadcastID: bid, Fingerprint: cert.Fingerprint(), BlobHash: cert.BlobHash(), TimeSalt: n / 600, SignedAt: n, ServerTime: n, SendBefore: min(n+60, (n/600+1)*600, cert.EXP), VerifyUntil: min((n/600+2)*600, cert.EXP), Signature: sig, APRSLine: line})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	f = testutil.Identity(t, n, server.URL+"/root", server.URL+"/inter")
	cert := f.Bundle.User.Cert()
	station := protocol.Station{SSID: 15, Latitude: "2505.12N", Longitude: "10242.22E", SymbolTable: "/", SymbolCode: "I", CountryCode: "CN", Name: "测试节点", Host: "voice.bi8syn.com", Port: 1883, FilterKM: 5000}
	info = signer.Identity{ServerID: serverID, BroadcastID: bid, EnrollmentID: id, Callsign: "BI8SYN", UID: 20, Fingerprint: cert.Fingerprint(), BlobHash: cert.BlobHash(), Blob: protocol.B64(cert.Blob()), Bundle: f.Bundle, SigningEnabled: true, ServerTime: n}
	raw, _ := os.ReadFile("../../configs/beacon.advanced.example.json")
	var cfg config.Config
	if e := jsonx.Decode(raw, &cfg); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	cfg.Applicant = &config.Applicant{Name: "Test Owner", Email: "owner@example.com", AuthorizationNote: "Synthetic test; do not treat as actual ownership"}
	cfg.CoordinatesConfirmed = true
	cfg.Callsign = "BI8SYN"
	cfg.Signer.BaseURL = server.URL
	cfg.Signer.AllowLoopbackHTTP = true
	cfg.Signer.CredentialFile = filepath.Join(dir, "token")
	cfg.APRS.PasscodeFile = filepath.Join(dir, "pass")
	cfg.StateDir = filepath.Join(dir, "state")
	cfg.Trust.Roots = []string{f.RootPin}
	cfg.Trust.CRLURLs = []string{server.URL + "/root", server.URL + "/inter"}
	cfg.Station = station
	os.WriteFile(cfg.Signer.CredentialFile, []byte("fmb1."+id+".AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"), 0600)
	os.WriteFile(cfg.APRS.PasscodeFile, []byte("12345"), 0600)
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	defer listener.Close()
	cfg.APRS.Host = "127.0.0.1"
	cfg.APRS.Port = uint64(listener.Addr().(*net.TCPAddr).Port)
	received := make(chan string, 1)
	go func() {
		conn, e := listener.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(3 * time.Second))
		reader := bufio.NewReader(conn)
		reader.ReadString('\n')
		fmt.Fprint(conn, "# logresp BI8SYN-15 verified, server TEST\r\n")
		line, _ := reader.ReadString('\n')
		received <- line
	}()
	svc, e := NewService(cfg)
	if e != nil {
		t.Fatal(e)
	}
	svc.now = func() time.Time { return now }
	if _, e = svc.Enroll(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Check(context.Background()); e != nil {
		t.Fatal(e)
	}
	output := filepath.Join(dir, "sas.json")
	if e = svc.ExportSAS(context.Background(), output); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Cycle(context.Background(), true); e == nil {
		t.Fatal("503 silently accepted")
	}
	if _, e = svc.Cycle(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	select {
	case line := <-received:
		if !strings.Contains(line, "FMO-V4,STATION,CERT:") || !strings.HasSuffix(line, "\r\n") {
			t.Fatal("APRS send failed")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("no APRS packet")
	}
	svc.Close()
	svc, e = NewService(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer svc.Close()
	svc.now = func() time.Time { return now }
	if _, e = svc.Cycle(context.Background(), true); e == nil {
		t.Fatal("restart repeated a recent broadcast")
	}
	if signatures != 1 {
		t.Fatal("wrong number of signatures")
	}
}

func TestScheduleCancellationAndBackoffBounds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := Wait(ctx, time.Hour); e != context.Canceled {
		t.Fatal("sleep ignored shutdown", e)
	}
	for i := 1; i < 20; i++ {
		delay := Backoff(i)
		if delay < time.Second || delay > 60*time.Second {
			t.Fatal("unbounded backoff", delay)
		}
	}
}
