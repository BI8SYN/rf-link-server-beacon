package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"github.com/BI8SYN/rf-link-server-beacon/internal/identity"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCertificateSnapshotIsDurableBeforePostAndSurvivesRotation(t *testing.T) {
	r, s := newStationRig(t)
	old := r.info
	r.replies = map[string]signer.Signature{}
	r.onSign = func(raw []byte) {
		var disk map[string]json.RawMessage
		json.Unmarshal(raw, &disk)
		var p struct {
			BroadcastID string           `json:"broadcastId"`
			Identity    *signer.Identity `json:"identity"`
		}
		if json.Unmarshal(disk["pending"], &p) != nil || p.Identity == nil || p.BroadcastID != localID || p.Identity.Blob != old.Blob || p.Identity.Fingerprint != old.Fingerprint || p.Identity.BlobHash != old.BlobHash {
			t.Error("POST preceded durable CERT, hash and resource binding")
		}
	}
	r.dropReply = true
	if _, e := s.Cycle(context.Background(), false); e == nil {
		t.Fatal("lost reply accepted")
	}
	s.Close()
	r.mu.Lock()
	r.onSign = nil
	r.mu.Unlock()
	r.rotateUserCertificate()
	r.mu.Lock()
	r.now = r.now.Add(31 * time.Second)
	r.info.ServerTime = uint64(r.now.Unix())
	r.mu.Unlock()
	r.cfg.Station.Name = "Changed after lost response"
	s = r.open()
	defer s.Close()
	_, e := s.Cycle(context.Background(), false)
	var retry *RetryError
	if !errors.As(e, &retry) || retry.Code != "recovered previous signing request; discarded old announcement" {
		t.Fatal("original CERT was not used to recover signed reply", e)
	}
	if out, e := s.Cycle(context.Background(), false); e != nil || !out.Verified || out.Fingerprint != r.info.Fingerprint {
		t.Fatal("next request did not select new CERT", e)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) != 3 || r.bodies[0] != r.bodies[1] || r.requests[0].RequestID == r.requests[2].RequestID {
		t.Fatal("certificate rotation rewrote retry intent")
	}
}

func TestPendingWithoutCertificateFailsClosedWithoutRebinding(t *testing.T) {
	r, s := newStationRig(t)
	defer s.Close()
	station := s.cfg.Station
	s.state.Pending = &SigningSnapshot{RequestID: localID, Station: &station}
	if e := s.store.Write(s.state); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Cycle(context.Background(), false); e == nil {
		t.Fatal("legacy uncertain request silently adopted current CERT")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) != 0 {
		t.Fatal("missing CERT snapshot reached signing endpoint")
	}
	if s.state.Pending == nil || s.state.Pending.RequestID != localID {
		t.Fatal("legacy request was silently replaced")
	}
}

func TestRevokedOriginalCertificateStopsRecoveryBeforePost(t *testing.T) {
	r, s := newStationRig(t)
	defer s.Close()
	r.replies = map[string]signer.Signature{}
	r.dropReply = true
	old := r.info
	oldUID := old.UID
	if _, e := s.Cycle(context.Background(), false); e == nil {
		t.Fatal("lost reply accepted")
	}
	r.rotateUserCertificate()
	r.mu.Lock()
	c := &r.f.IntermediateCRL
	c.Number++
	c.ThisUpdate++
	c.Entries = []identity.Entry{{UID: &oldUID, Fingerprint: old.Fingerprint, RevokedAt: c.ThisUpdate, Reason: 1}}
	c.Signature = protocol.B64(ed25519.Sign(r.f.IntermediateKey, c.TBS()))
	r.mu.Unlock()
	if _, e := s.Cycle(context.Background(), false); e == nil {
		t.Fatal("revoked original certificate used to recover")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) != 1 {
		t.Fatal("revoked snapshot submitted another signing request")
	}
}

func TestCorruptedCertificateSnapshotCannotBeReplacedByLiveIdentity(t *testing.T) {
	for _, field := range []string{"certBlob", "certBlobHash", "certificateFingerprint", "broadcastId"} {
		t.Run(field, func(t *testing.T) {
			r, s := newStationRig(t)
			r.dropReply = true
			if _, e := s.Cycle(context.Background(), false); e == nil {
				t.Fatal("lost reply accepted")
			}
			s.Close()
			path := filepath.Join(r.cfg.StateDir, "state.json")
			raw, _ := os.ReadFile(path)
			var state map[string]any
			json.Unmarshal(raw, &state)
			p := state["pending"].(map[string]any)
			if p["identity"] == nil {
				t.Fatal("certificate snapshot was not persisted")
			}
			p["identity"].(map[string]any)[field] = "invalid"
			raw, _ = json.Marshal(state)
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			s = r.open()
			defer s.Close()
			if _, e := s.Cycle(context.Background(), false); e == nil {
				t.Fatal("corrupt snapshot accepted")
			}
			r.mu.Lock()
			defer r.mu.Unlock()
			if len(r.requests) != 1 {
				t.Fatal("corrupt snapshot reached signer")
			}
		})
	}
}

func (r *stationRig) rotateUserCertificate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	pub, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		r.t.Fatal(e)
	}
	u := &r.f.Bundle.User
	u.Subject.PublicKey = protocol.B64(pub)
	u.Signature = protocol.B64(ed25519.Sign(r.f.IntermediateKey, u.Cert().TBS()))
	u.Fingerprint = u.Cert().Fingerprint()
	r.f.UserKey = key
	c := u.Cert()
	r.info.Bundle = r.f.Bundle
	r.info.Blob = protocol.B64(c.Blob())
	r.info.Fingerprint = c.Fingerprint()
	r.info.BlobHash = c.BlobHash()
}
