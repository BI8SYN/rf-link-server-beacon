package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"testing"
)

func TestCRLExpiryRollbackAndRevocation(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	c := CRL{Type: "intermediateCRL", IssuerSN: 2, Number: 4, ThisUpdate: 500, NextUpdate: 1500, Entries: []Entry{}, Algorithm: "Ed25519"}
	c.Signature = protocol.B64(ed25519.Sign(key, c.TBS()))
	target := protocol.Hash([]byte("user"))
	mark, e := CheckCRL(c, pub, 2, 200, 2000, 10, 100, 20, target, 1000, Mark{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = CheckCRL(c, pub, 2, 200, 2000, 10, 100, 20, target, 1500, Mark{}); e == nil {
		t.Fatal("expired CRL accepted")
	}
	old := c
	old.Number = 3
	old.Signature = protocol.B64(ed25519.Sign(key, old.TBS()))
	if _, e = CheckCRL(old, pub, 2, 200, 2000, 10, 100, 20, target, 1000, mark); e == nil {
		t.Fatal("rollback accepted")
	}
	c.Entries = []Entry{{UID: ptr(20), Fingerprint: target, RevokedAt: 400, Reason: 0}}
	c.Signature = protocol.B64(ed25519.Sign(key, c.TBS()))
	if _, e = CheckCRL(c, pub, 2, 200, 2000, 10, 100, 20, target, 1000, Mark{}); e == nil {
		t.Fatal("revocation accepted")
	}
	c.Entries[0].Fingerprint = protocol.Hash([]byte("other"))
	c.Signature = protocol.B64(ed25519.Sign(key, c.TBS()))
	if _, e = CheckCRL(c, pub, 2, 200, 2000, 10, 100, 20, target, 1000, Mark{}); e != nil {
		t.Fatal("UID alone incorrectly revoked unrelated certificate", e)
	}
	if _, e = CheckCRL(c, pub, 2, 200, 2000, 10, 100, 20, target, 1000, mark); e == nil {
		t.Fatal("same version conflict accepted")
	}
}
func ptr(n uint64) *uint64 { return &n }

func TestRejectSignedOutOfSchemaCRL(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	for _, reason := range []uint64{5, 255} {
		c := CRL{Type: "intermediateCRL", IssuerSN: 2, Number: 1, ThisUpdate: 500, NextUpdate: 1500, Entries: []Entry{{UID: ptr(21), Fingerprint: protocol.Hash([]byte("other")), RevokedAt: 400, Reason: reason}}, Algorithm: "Ed25519"}
		c.Signature = protocol.B64(ed25519.Sign(key, c.TBS()))
		if _, e := CheckCRL(c, pub, 2, 200, 2000, 10, 100, 20, protocol.Hash([]byte("target")), 1000, Mark{}); e == nil {
			t.Fatalf("reason outside protocol enum accepted: %d", reason)
		}
	}
}
