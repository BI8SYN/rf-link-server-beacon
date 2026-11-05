package app

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"github.com/BI8SYN/rf-link-server-beacon/internal/jsonx"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"github.com/BI8SYN/rf-link-server-beacon/internal/testutil"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

const localID = "95b976da-d670-44ec-bb72-b876ad05f134"
const localToken = "fmb1.550e8400-e29b-41d4-a716-446655440000.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

type stationRig struct {
	mu        sync.Mutex
	now       time.Time
	f         testutil.Fixture
	info      signer.Identity
	cfg       config.Config
	requests  []signer.SignRequest
	bodies    []string
	failCode  string
	dropReply bool
	onSign    func([]byte)
	replies   map[string]signer.Signature
	t         *testing.T
}

func newStationRig(t *testing.T) (*stationRig, *Service) {
	t.Helper()
	r := &stationRig{t: t, now: time.Unix(time.Now().Unix()/600*600+10, 0)}
	server := httptest.NewServer(http.HandlerFunc(r.handle))
	t.Cleanup(server.Close)
	n := uint64(r.now.Unix())
	r.f = testutil.Identity(t, n, server.URL+"/root", server.URL+"/inter")
	c := r.f.Bundle.User.Cert()
	r.info = signer.Identity{ServerID: localID, EnrollmentID: localID, BroadcastID: localID, Callsign: c.Callsign, UID: c.UID, Fingerprint: c.Fingerprint(), BlobHash: c.BlobHash(), Blob: protocol.B64(c.Blob()), Bundle: r.f.Bundle, SigningEnabled: true, ServerTime: n}
	r.cfg = applicationConfig(t, t.TempDir(), server.URL)
	r.cfg.Callsign = c.Callsign
	r.cfg.Station.CountryCode = "CN"
	r.cfg.Trust.Roots = []string{r.f.RootPin}
	r.cfg.Trust.CRLURLs = []string{server.URL + "/root", server.URL + "/inter"}
	s, e := NewService(r.cfg)
	if e != nil {
		t.Fatal(e)
	}
	s.state.ServerID = localID
	s.state.EnrollmentID = localID
	s.state.BroadcastID = localID
	s.state.Credential = &signer.Credential{RequestID: localID, EnrollmentID: localID, BroadcastID: localID, ServerID: localID, CredentialID: "550e8400-e29b-41d4-a716-446655440000", MachineToken: localToken, ExpiresAt: n + 60000, RetryUntil: n + 900}
	if e = s.store.Write(s.state); e != nil {
		t.Fatal(e)
	}
	s.Close()
	return r, r.open()
}
func (r *stationRig) open() *Service {
	r.t.Helper()
	s, e := NewService(r.cfg)
	if e != nil {
		r.t.Fatal(e)
	}
	s.now = func() time.Time { return r.now }
	return s
}
func (r *stationRig) handle(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	switch {
	case req.URL.Path == "/root":
		enc.Encode(r.f.RootCRL)
	case req.URL.Path == "/inter":
		enc.Encode(r.f.IntermediateCRL)
	case req.URL.Path == "/api/v1/server-broadcast/identities/"+localID:
		enc.Encode(r.info)
	case strings.HasSuffix(req.URL.Path, "/station-signatures"):
		raw, _ := io.ReadAll(req.Body)
		var in signer.SignRequest
		if e := jsonx.Decode(raw, &in); e != nil {
			r.t.Error("wrong signature shape", e)
			w.WriteHeader(422)
			return
		}
		var disk State
		stateRaw, _ := os.ReadFile(filepath.Join(r.cfg.StateDir, "state.json"))
		if jsonx.Decode(stateRaw, &disk) != nil || disk.Pending == nil || disk.Pending.Station == nil || disk.Pending.RequestID != in.RequestID || *disk.Pending.Station != in.Station || disk.Pending.Online != in.Online || disk.Pending.Peak != in.Peak {
			r.t.Error("HTTP preceded durable complete request snapshot")
		}
		r.requests = append(r.requests, in)
		r.bodies = append(r.bodies, string(raw))
		if r.onSign != nil {
			r.onSign(stateRaw)
		}
		var cached *signer.Signature
		if r.replies != nil {
			value, ok := r.replies[in.RequestID]
			if !ok {
				value = r.signature(in)
				r.replies[in.RequestID] = value
			}
			cached = &value
		}
		if r.dropReply {
			r.dropReply = false
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		if r.failCode != "" {
			code := r.failCode
			r.failCode = ""
			status := 409
			if code == "invalid_request" {
				status = 422
			}
			w.WriteHeader(status)
			enc.Encode(map[string]any{"error": map[string]string{"code": code, "message": "test"}, "correlationId": "test"})
			return
		}
		if cached != nil {
			cached.ServerTime = uint64(r.now.Unix())
			enc.Encode(cached)
		} else {
			enc.Encode(r.signature(in))
		}
	default:
		r.t.Error("unexpected request; identities must not be reenrolled or credentials rotated")
		http.NotFound(w, req)
	}
}

func (r *stationRig) signature(in signer.SignRequest) signer.Signature {
	n := uint64(r.now.Unix())
	c := r.f.Bundle.User.Cert()
	sig := protocol.B64(ed25519.Sign(r.f.UserKey, protocol.StationTBS(c, in.Station, in.Online, in.Peak, n/600)))
	line, e := protocol.Line(c, in.Station, in.Online, in.Peak, sig)
	if e != nil {
		r.t.Error(e)
	}
	return signer.Signature{RequestID: in.RequestID, BroadcastID: localID, Fingerprint: c.Fingerprint(), BlobHash: c.BlobHash(), TimeSalt: n / 600, SignedAt: n, ServerTime: n, SendBefore: n + 60, VerifyUntil: (n/600 + 2) * 600, Signature: sig, APRSLine: line}
}

func TestStationSnapshotSurvivesLostReplyRestartAndConfigChange(t *testing.T) {
	r, s := newStationRig(t)
	r.dropReply = true
	if _, e := s.Cycle(context.Background(), false); e == nil {
		t.Fatal("lost reply accepted")
	}
	s.Close()
	r.cfg.Station.Name = "New station"
	r.cfg.Station.Host = "new.example.com"
	r.cfg.Station.Port = 8883
	r.cfg.Station.SSID = 12
	r.cfg.Station.Latitude = "2506.25N"
	r.cfg.Station.Longitude = "10242.50E"
	r.cfg.Station.FilterKM = 42
	r.cfg.Statistics.Online = 2
	r.cfg.Statistics.Peak = 6
	s = r.open()
	defer s.Close()
	_, e := s.Cycle(context.Background(), false)
	var retry *RetryError
	if !errors.As(e, &retry) || retry.Code != "recovered previous signing request; discarded old announcement" {
		t.Fatal("recovered packet must be verified then discarded", e)
	}
	if out, e := s.Cycle(context.Background(), false); e != nil || !out.Verified || out.Published {
		t.Fatal("new local configuration failed to sign", e)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) != 3 || r.bodies[0] != r.bodies[1] || r.requests[2].RequestID == r.requests[0].RequestID || r.requests[2].Station != r.cfg.Station || r.requests[2].Online != 2 || r.requests[2].Peak != 6 {
		t.Fatal("retry snapshot or next configuration incorrect")
	}
	if s.state.Credential.MachineToken != localToken || s.state.EnrollmentID != localID {
		t.Fatal("configuration change rotated identity")
	}
}

func TestUncertainSigningRequestDoesNotAdoptChangedConfiguration(t *testing.T) {
	r, s := newStationRig(t)
	defer s.Close()
	r.dropReply = true
	if _, e := s.Cycle(context.Background(), false); e == nil {
		t.Fatal("lost reply accepted")
	}
	// Elapsed local time alone must not replace an uncertain request ID.
	s.state.PendingSince = uint64(r.now.Add(-8 * 24 * time.Hour).Unix())
	s.cfg.Station.Name = "Changed while request uncertain"
	s.cfg.Statistics.Online = 1
	s.cfg.Statistics.Peak = 4
	if out, e := s.Cycle(context.Background(), false); e != nil || !out.Verified {
		t.Fatal("original request could not be recovered", e)
	}
	if out, e := s.Cycle(context.Background(), false); e != nil || !out.Verified {
		t.Fatal("next request could not use new configuration", e)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) != 3 || r.bodies[0] != r.bodies[1] || r.requests[2].RequestID == r.requests[0].RequestID || r.requests[2].Station != s.cfg.Station || r.requests[2].Online != 1 || r.requests[2].Peak != 4 {
		t.Fatal("uncertain request body changed or completed request reused")
	}
}

func TestExpiredSignatureGetsNewIDAndCurrentConfigBut422DoesNotFallback(t *testing.T) {
	for _, code := range []string{"signature_expired", "signature_window_ending", "invalid_request"} {
		t.Run(code, func(t *testing.T) {
			r, s := newStationRig(t)
			defer s.Close()
			r.failCode = code
			if _, e := s.Cycle(context.Background(), false); e == nil {
				t.Fatal("API failure accepted")
			}
			r.mu.Lock()
			first := r.requests[0]
			r.mu.Unlock()
			if code == "invalid_request" {
				if s.state.Pending == nil || s.state.Pending.RequestID != first.RequestID {
					t.Fatal("422 changed request intent")
				}
				r.mu.Lock()
				defer r.mu.Unlock()
				if len(r.requests) != 1 {
					t.Fatal("422 silently fell back")
				}
				return
			}
			s.cfg.Station.Name = "Updated after expiry"
			if out, e := s.Cycle(context.Background(), false); e != nil || !out.Verified {
				t.Fatal(e)
			}
			r.mu.Lock()
			defer r.mu.Unlock()
			if len(r.requests) != 2 || r.requests[1].RequestID == first.RequestID || r.requests[1].Station.Name != s.cfg.Station.Name {
				t.Fatal("expiry did not use a new ID and current configuration")
			}
		})
	}
}

func TestLegacyCutoverDiscardsOnlyPendingProfileSignature(t *testing.T) {
	r, s := newStationRig(t)
	v := uint64(1)
	legacyStation := r.cfg.Station
	s.state.Enrollment = &signer.Enrollment{RequestID: localID, Callsign: r.cfg.Callsign, Label: "Original identity", PurposeNote: "Original purpose", Applicant: r.cfg.Applicant, Station: &legacyStation}
	s.state.ApplicationAccessToken = "fmba1." + localID + ".AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	s.state.Pending = &SigningSnapshot{RequestID: localID, LegacyProfileVersion: &v, Online: 0, Peak: 0}
	s.state.ProfileVersion = 1
	s.state.LastAttempt = uint64(r.now.Unix()) - 600
	s.state.Fingerprint = r.info.Fingerprint
	before := s.state
	if e := s.store.Write(s.state); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s = r.open()
	defer s.Close()
	if s.state.Pending != nil {
		t.Fatal("old profile signing request survived cutover")
	}
	after := s.state
	before.Pending = nil
	before.PendingSince = 0
	if !reflect.DeepEqual(before, after) {
		t.Fatal("cutover changed identity, secrets, immutable application or replay protection")
	}
	var disk State
	if e := s.store.Read(&disk); e != nil || disk.Pending != nil || disk.ApplicationAccessToken != before.ApplicationAccessToken {
		t.Fatal("migration was not durable", e)
	}
}

func TestSASExportUsesLocalEndpointWithExistingIdentity(t *testing.T) {
	r, s := newStationRig(t)
	defer s.Close()
	s.cfg.Station.Host = "local.example.com"
	s.cfg.Station.Port = 8883
	path := filepath.Join(t.TempDir(), "sas.json")
	if e := s.ExportSAS(context.Background(), path); e != nil {
		t.Fatal(e)
	}
	var out struct {
		Mqtt struct {
			Host string
			Port uint64
		}
		Server struct {
			Uid             uint64
			CertFingerprint string
		}
	}
	raw, _ := os.ReadFile(path)
	if e := json.Unmarshal(raw, &out); e != nil {
		t.Fatal(e)
	}
	if out.Mqtt.Host != s.cfg.Station.Host || out.Mqtt.Port != 8883 || out.Server.Uid != r.info.UID || out.Server.CertFingerprint != r.info.Fingerprint {
		t.Fatal("SAS did not combine local endpoint with verified identity")
	}
}
