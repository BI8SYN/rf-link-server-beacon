// Package testutil creates independent, temporary identities for offline tests only.
package testutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"github.com/BI8SYN/rf-link-server-beacon/internal/identity"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"testing"
)

type Fixture struct {
	Bundle                            identity.Bundle
	RootPin                           string
	RootKey, IntermediateKey, UserKey ed25519.PrivateKey
	RootCRL, IntermediateCRL          identity.CRL
}

func Identity(t *testing.T, now uint64, rootURL, interURL string) Fixture {
	t.Helper()
	rp, rk, _ := ed25519.GenerateKey(rand.Reader)
	ip, ik, _ := ed25519.GenerateKey(rand.Reader)
	up, uk, _ := ed25519.GenerateKey(rand.Reader)
	var b identity.Bundle
	r := &b.Root
	r.SN = 1
	r.Type = "rootCA"
	r.Issuer.Name = "TEST ROOT"
	r.Subject.Name = "TEST ROOT"
	r.Subject.PublicKey = protocol.B64(rp)
	r.Extensions.IsCA = true
	r.Extensions.PathLen = 1
	r.Extensions.CRL = rootURL
	r.Extensions.KeyID = "test-root"
	r.IAT = now - 1000
	r.EXP = now + 86400
	r.Algorithm = "Ed25519"
	r.Signature = protocol.B64(ed25519.Sign(rk, r.TBS()))
	i := &b.Intermediate
	i.SN = 2
	i.Type = "intermediateCA"
	i.Issuer.SN = 1
	i.Issuer.Name = "TEST ROOT"
	i.Issuer.PublicKey = r.Subject.PublicKey
	i.Subject.Name = "TEST INTERMEDIATE"
	i.Subject.PublicKey = protocol.B64(ip)
	i.Extensions.IsCA = true
	i.Extensions.KeyID = "test-intermediate"
	i.Extensions.CRL = interURL
	i.Extensions.UIDRange.Start = 10
	i.Extensions.UIDRange.End = 100
	i.Extensions.Countries = []string{"CN"}
	i.IAT = now - 900
	i.EXP = now + 80000
	i.Algorithm = "Ed25519"
	i.Signature = protocol.B64(ed25519.Sign(rk, i.TBS()))
	u := &b.User
	u.Type = "userCert"
	u.IssuerSN = 2
	u.Subject.Callsign = "BI8SYN"
	u.Subject.UID = 20
	u.Subject.PublicKey = protocol.B64(up)
	u.IAT = now - 800
	u.EXP = now + 70000
	u.Algorithm = "Ed25519"
	u.Signature = protocol.B64(ed25519.Sign(ik, u.Cert().TBS()))
	u.Fingerprint = u.Cert().Fingerprint()
	rc := identity.CRL{Type: "rootCRL", IssuerSN: 1, Number: 1, ThisUpdate: now - 100, NextUpdate: now + 3600, Entries: []identity.Entry{}, Algorithm: "Ed25519"}
	rc.Signature = protocol.B64(ed25519.Sign(rk, rc.TBS()))
	ic := identity.CRL{Type: "intermediateCRL", IssuerSN: 2, Number: 1, ThisUpdate: now - 100, NextUpdate: now + 3600, Entries: []identity.Entry{}, Algorithm: "Ed25519"}
	ic.Signature = protocol.B64(ed25519.Sign(ik, ic.TBS()))
	return Fixture{b, protocol.Hash(r.TBS()), rk, ik, uk, rc, ic}
}
