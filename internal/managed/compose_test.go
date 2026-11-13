package managed

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"github.com/BI8SYN/rf-link-server-beacon/internal/testutil"
)

// The Compose smoke test runs this fixture inside Beacon's network namespace.
// Every signing, CRL and APRS endpoint is local and uses disposable test keys.
func TestComposeFixture(t *testing.T) {
	if os.Getenv("BEACON_COMPOSE_TEST") != "1" {
		t.Skip("requires isolated Compose volumes")
	}
	const origin = "http://127.0.0.1:19080"
	const id = "95b976da-d670-44ec-bb72-b876ad05f134"
	const address = "voice.example.com"
	now := uint64(time.Now().Unix())
	f := testutil.Identity(t, now, origin+"/root", origin+"/inter")
	cert := f.Bundle.User.Cert()
	raw, err := json.Marshal(f.Bundle.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("/exchange/roots/fixture.json", raw, 0644); err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	cfg.Schema = 1
	cfg.CoordinatesConfirmed = true
	cfg.Callsign = cert.Callsign
	cfg.Label = "Synthetic test"
	cfg.PurposeNote = "Isolated Compose validation"
	cfg.Applicant = &config.Applicant{Name: "Test Owner", Email: "owner@example.com", AuthorizationNote: "Synthetic test"}
	cfg.StateDir = "/data/state"
	cfg.Signer.BaseURL = origin
	cfg.Signer.CredentialFile = "/data/credential"
	cfg.Signer.AllowLoopbackHTTP = true
	cfg.Trust.Roots = []string{f.RootPin}
	cfg.Trust.CRLURLs = []string{origin + "/root", origin + "/inter"}
	cfg.Station = protocol.Station{SSID: 15, Latitude: "2505.12N", Longitude: "10242.22E", SymbolTable: "/", SymbolCode: "I", CountryCode: "CN", Name: "Synthetic", Host: address, Port: 1883, FilterKM: 5000}
	cfg.APRS.Host = "127.0.0.1"
	cfg.APRS.Port = 19480
	cfg.APRS.Interval = 600
	raw, err = json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("/data/config.json", raw, 0600); err != nil {
		t.Fatal(err)
	}

	// Proof used to verify that real SAS permits a correctly signed login.
	// Its timestamp is refreshed by the HTTP endpoint before each client probe.
	var mu sync.Mutex
	var enrollment string
	handle := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		n := uint64(time.Now().Unix())
		switch {
		case r.URL.Path == "/root":
			enc.Encode(f.RootCRL)
		case r.URL.Path == "/inter":
			enc.Encode(f.IntermediateCRL)
		case r.URL.Path == "/test/login":
			fingerprint, _ := protocol.Decode64(cert.Fingerprint(), 32)
			proof := protocol.Encode([]any{"FMO", uint64(4), "serverAuthorizerReqHttp", cert.UID, cert.Callsign, cert.UID, "super", address, uint64(1883), fingerprint, n, fingerprint})
			payload, _ := json.Marshal(map[string]any{"certPackage": map[string]any{"intermediateCert": f.Bundle.Intermediate, "userCert": f.Bundle.User}, "targetCallsign": cert.Callsign, "targetUID": cert.UID, "role": "super", "targetUrl": address, "targetPort": 1883, "serverFingerprint": cert.Fingerprint(), "timestamp": n, "proof": map[string]string{"signature": protocol.B64(ed25519.Sign(f.UserKey, proof))}})
			enc.Encode(map[string]string{"username": cert.Callsign, "password": protocol.B64(payload)})
		case r.URL.Path == "/api/v1/server-broadcast/enrollments" && r.Method == "POST":
			var req signer.Enrollment
			if json.NewDecoder(r.Body).Decode(&req) != nil || req.Callsign != cert.Callsign || req.Station != nil {
				w.WriteHeader(422)
				return
			}
			enrollment = req.RequestID
			w.WriteHeader(201)
			enc.Encode(signer.Created{ServerID: id, EnrollmentID: enrollment, BroadcastID: id, State: "pending_review"})
		case strings.HasSuffix(r.URL.Path, "/credentials"):
			var req struct {
				RequestID string `json:"requestId"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			enc.Encode(signer.Credential{RequestID: req.RequestID, EnrollmentID: enrollment, BroadcastID: id, ServerID: id, CredentialID: id, MachineToken: "fmb1." + id + ".AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", ExpiresAt: now + 3600, RetryUntil: n + 600})
		case strings.HasPrefix(r.URL.Path, "/api/v1/server-broadcast/enrollments/"):
			status := signer.Status{ServerID: id, EnrollmentID: enrollment, BroadcastID: id, State: "pending_review", PollAfter: 1}
			if _, e := os.Stat("/data/test-approved"); e == nil {
				fp := cert.Fingerprint()
				status.Fingerprint = &fp
				status.State = "ready"
			}
			enc.Encode(status)
		case strings.HasSuffix(r.URL.Path, "/station-signatures"):
			var req signer.SignRequest
			json.NewDecoder(r.Body).Decode(&req)
			sig := protocol.B64(ed25519.Sign(f.UserKey, protocol.StationTBS(cert, req.Station, req.Online, req.Peak, n/600)))
			enc.Encode(signer.Signature{RequestID: req.RequestID, BroadcastID: id, Fingerprint: cert.Fingerprint(), BlobHash: cert.BlobHash(), TimeSalt: n / 600, SignedAt: n, ServerTime: n, SendBefore: min(n+60, (n/600+1)*600), VerifyUntil: (n/600 + 2) * 600, Signature: sig})
		case strings.HasSuffix(r.URL.Path, "/identities/"+id):
			enc.Encode(signer.Identity{ServerID: id, BroadcastID: id, EnrollmentID: enrollment, Callsign: cert.Callsign, UID: cert.UID, Fingerprint: cert.Fingerprint(), BlobHash: cert.BlobHash(), Blob: protocol.B64(cert.Blob()), Bundle: f.Bundle, SigningEnabled: true, ServerTime: n})
		default:
			http.NotFound(w, r)
		}
	})
	// The login probe runs inside the isolated Beacon container.
	login, err := net.Listen("tcp", "127.0.0.1:19081")
	if err != nil {
		t.Fatal(err)
	}
	defer login.Close()
	go http.Serve(login, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test/login" {
			http.NotFound(w, r)
			return
		}
		handle.ServeHTTP(w, r)
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:19080")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go http.Serve(listener, handle)
	aprs, err := net.Listen("tcp", "127.0.0.1:19480")
	if err != nil {
		t.Fatal(err)
	}
	defer aprs.Close()
	go func() {
		for {
			c, e := aprs.Accept()
			if e != nil {
				return
			}
			func() {
				defer c.Close()
				c.SetDeadline(time.Now().Add(3 * time.Second))
				in := bufio.NewReader(c)
				in.ReadString('\n')
				io.WriteString(c, "# logresp "+cert.Callsign+"-15 verified, server TEST\r\n")
				line, _ := in.ReadString('\n')
				if strings.Contains(line, "IFMO-V4,STATION,CERT:") {
					os.WriteFile("/data/test-aprs", []byte(line), 0600)
				}
			}()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if e := Run(ctx, os.Stdout); e != nil {
		t.Fatal(fmt.Errorf("managed stack: %w", e))
	}
}

// TestComposeMQTTProbe is invoked inside the isolated Compose network.
func TestComposeMQTTProbe(t *testing.T) {
	expected, enabled := os.LookupEnv("BEACON_PROBE_EXPECT")
	if !enabled {
		t.Skip("requires Compose fixture")
	}
	user, password := "invalid", "invalid"
	if expected == "allow" {
		r, e := http.Get("http://127.0.0.1:19081/test/login")
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		var login map[string]string
		if e = json.NewDecoder(r.Body).Decode(&login); e != nil {
			t.Fatal(e)
		}
		user, password = login["username"], login["password"]
	}
	c, e := net.DialTimeout("tcp", "127.0.0.1:1883", 2*time.Second)
	if e != nil {
		if expected == "closed" || expected == "blocked" {
			return
		}
		t.Fatal(e)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	payload := []byte{0, 4, 'M', 'Q', 'T', 'T', 4, 0xc2, 0, 5}
	for _, s := range []string{fmt.Sprintf("probe-%d", time.Now().UnixNano()), user, password} {
		payload = append(payload, byte(len(s)>>8), byte(len(s)))
		payload = append(payload, s...)
	}
	packet := []byte{0x10}
	n := len(payload)
	for {
		digit := byte(n % 128)
		n /= 128
		if n > 0 {
			digit |= 128
		}
		packet = append(packet, digit)
		if n == 0 {
			break
		}
	}
	c.Write(append(packet, payload...))
	reply := make([]byte, 4)
	_, e = io.ReadFull(c, reply)
	switch expected {
	case "closed":
		if e == nil {
			t.Fatalf("unexpected reply %x", reply)
		}
	case "blocked":
		if e == nil && reply[3] == 0 {
			t.Fatal("unauthenticated MQTT accepted")
		}
	case "deny":
		if e != nil || (reply[3] != 4 && reply[3] != 5) {
			t.Fatalf("expected denial, got %x %v", reply, e)
		}
	case "allow":
		if e != nil || reply[0] != 0x20 || reply[3] != 0 {
			t.Fatalf("expected login, got %x %v", reply, e)
		}
	default:
		t.Fatal("invalid probe expectation")
	}
}
