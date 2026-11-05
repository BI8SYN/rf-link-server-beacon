package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type claimRig struct {
	mu                          sync.Mutex
	server                      *httptest.Server
	now                         uint64
	application, access         string
	claims                      map[string]signer.Credential
	requests                    []string
	lose, expire, deny, pending bool
}

func newClaimRig(t *testing.T) (*claimRig, config.Config, *Service) {
	t.Helper()
	rig := &claimRig{now: uint64(time.Now().Unix()), claims: map[string]signer.Credential{}}
	rig.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rig.mu.Lock()
		defer rig.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		const id = "550e8400-e29b-41d4-a716-446655440000"
		if r.URL.Path == "/api/v1/server-broadcast/enrollments" {
			var request signer.Enrollment
			json.NewDecoder(r.Body).Decode(&request)
			rig.application = request.RequestID
			rig.access = r.Header.Get("Authorization")
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(signer.Created{ServerID: id, EnrollmentID: rig.application, BroadcastID: id, State: "pending_review"})
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/credentials") {
			t.Error("unexpected fixture operation")
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != rig.access || !strings.HasPrefix(rig.access, "Bearer fmba1.") {
			t.Error("machine credential used on claim endpoint")
		}
		var body struct {
			RequestID string `json:"requestId"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		rig.requests = append(rig.requests, body.RequestID)
		errorResponse := func(status int, code string) {
			w.WriteHeader(status)
			fmt.Fprintf(w, `{"error":{"code":%q,"message":"test"},"correlationId":"test"}`, code)
		}
		if rig.deny {
			errorResponse(403, "registration_disabled")
			return
		}
		if rig.pending {
			w.Header().Set("Retry-After", "120")
			errorResponse(409, "credential_recovery_pending")
			return
		}
		if rig.expire {
			rig.expire = false
			errorResponse(409, "credential_delivery_expired")
			return
		}
		out, ok := rig.claims[body.RequestID]
		if !ok {
			credentialID, _ := signer.NewID()
			secret := make([]byte, 32)
			secret[0] = byte(len(rig.claims) + 1)
			out = signer.Credential{RequestID: body.RequestID, EnrollmentID: rig.application, BroadcastID: id, ServerID: id, CredentialID: credentialID, MachineToken: "fmb1." + credentialID + "." + protocol.B64(secret), ExpiresAt: rig.now + 3*86400, RetryUntil: rig.now + 900}
			rig.claims[body.RequestID] = out
		}
		if rig.lose {
			rig.lose = false
			connection, _, _ := w.(http.Hijacker).Hijack()
			connection.Close()
			return
		}
		json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(rig.server.Close)
	cfg := applicationConfig(t, filepath.Join(t.TempDir(), "data"), rig.server.URL)
	svc, e := NewService(cfg)
	if e != nil {
		t.Fatal(e)
	}
	svc.now = rig.clock
	if _, e = svc.Enroll(context.Background()); e != nil {
		svc.Close()
		t.Fatal(e)
	}
	return rig, cfg, svc
}
func (r *claimRig) clock() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return time.Unix(int64(r.now), 0)
}
func (r *claimRig) counts() (int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests), len(r.claims)
}
func TestLostCredentialDeliveryRecoversSameIDAfterRestartAndRenewsOnce(t *testing.T) {
	r, cfg, s := newClaimRig(t)
	r.mu.Lock()
	r.lose = true
	r.mu.Unlock()
	if e := s.ensureCredential(context.Background(), false); e == nil {
		t.Fatal("lost delivery accepted")
	}
	id := s.state.ClaimRequestID
	if id == "" || s.state.Credential != nil {
		t.Fatal("claim intent not saved before receiving secret")
	}
	s.Close()
	s, e := NewService(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	s.now = r.clock
	if e = s.ensureCredential(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	requests, issued := r.counts()
	if requests != 2 || issued != 1 || s.state.Credential.RequestID != id || s.state.ClaimRequestID != "" {
		t.Fatal("claim replay rotated identity or lost delivery")
	}
	old := s.state.Credential.MachineToken
	if e = s.ensureCredential(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	requests, _ = r.counts()
	if requests != 2 {
		t.Fatal("normal operation unnecessarily rotated credentials")
	}
	r.mu.Lock()
	r.now += 2*86400 + 1
	r.mu.Unlock()
	if e = s.ensureCredential(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	_, issued = r.counts()
	if issued != 2 || s.state.Credential.MachineToken == old {
		t.Fatal("near-expiry credential not renewed")
	}
}
func TestExpiredDeliveryAllocatesNewClaimOnlyAfterServerRejection(t *testing.T) {
	r, _, s := newClaimRig(t)
	defer s.Close()
	r.mu.Lock()
	r.lose = true
	r.mu.Unlock()
	s.ensureCredential(context.Background(), false)
	old := s.state.ClaimRequestID
	r.mu.Lock()
	r.expire = true
	r.mu.Unlock()
	var retry *RetryError
	if e := s.ensureCredential(context.Background(), false); !errors.As(e, &retry) || retry.Code != "credential_delivery_expired" {
		t.Fatal("expired receipt not handled", e)
	}
	if s.state.ClaimRequestID != "" {
		t.Fatal("expired claim retained")
	}
	if e := s.ensureCredential(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	if s.state.Credential.RequestID == old {
		t.Fatal("expired delivery ID reused")
	}
}
func TestRecoveryPendingHonorsRetryAfterAndRevocationStops(t *testing.T) {
	r, _, s := newClaimRig(t)
	defer s.Close()
	if e := s.ensureCredential(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	s.state.CredentialRenewAt = 0
	r.mu.Lock()
	r.pending = true
	r.mu.Unlock()
	if e := s.ensureCredential(context.Background(), false); e != nil {
		t.Fatal("valid current token needlessly discarded during recovery window", e)
	}
	before, _ := r.counts()
	if e := s.ensureCredential(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	after, _ := r.counts()
	if after != before {
		t.Fatal("Retry-After not honored")
	}
	var retry *RetryError
	if e := s.ensureCredential(context.Background(), true); !errors.As(e, &retry) {
		t.Fatal("invalid old token reused during forced recovery")
	}
	r.mu.Lock()
	r.pending = false
	r.deny = true
	r.now += 121
	r.mu.Unlock()
	var api *signer.APIError
	if e := s.ensureCredential(context.Background(), true); !errors.As(e, &api) || api.Status != 403 {
		t.Fatal("revoked authorization recovered automatically", e)
	}
}
