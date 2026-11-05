// Package identity verifies FMO JSON certificates against explicitly pinned roots.
package identity

import (
	"crypto/ed25519"
	"errors"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"sort"
	"strings"
	"unicode/utf8"
)

type Root struct {
	SN     uint64 `json:"sn"`
	Type   string `json:"type"`
	Issuer struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"issuer"`
	Subject struct {
		Name      string `json:"name"`
		PublicKey string `json:"publicKey"`
	} `json:"subject"`
	Extensions struct {
		IsCA    bool   `json:"isCA"`
		PathLen uint64 `json:"pathLen"`
		CRL     string `json:"crl"`
		License string `json:"license"`
		KeyID   string `json:"keyId"`
	} `json:"extensions"`
	IAT       uint64 `json:"iat"`
	EXP       uint64 `json:"exp"`
	Algorithm string `json:"signatureAlgorithm"`
	Signature string `json:"signature"`
}
type Intermediate struct {
	SN     uint64 `json:"sn"`
	Type   string `json:"type"`
	Issuer struct {
		SN        uint64 `json:"sn"`
		Name      string `json:"name"`
		PublicKey string `json:"publicKey"`
	} `json:"issuer"`
	Subject struct {
		Name      string `json:"name"`
		Email     string `json:"email"`
		PublicKey string `json:"publicKey"`
	} `json:"subject"`
	Extensions struct {
		IsCA     bool   `json:"isCA"`
		PathLen  uint64 `json:"pathLen"`
		KeyID    string `json:"keyId"`
		CRL      string `json:"crl"`
		License  string `json:"license"`
		UIDRange struct {
			Start uint64 `json:"start"`
			End   uint64 `json:"end"`
		} `json:"uidRange"`
		Countries []string `json:"issuingCountries"`
	} `json:"extensions"`
	IAT       uint64 `json:"iat"`
	EXP       uint64 `json:"exp"`
	Algorithm string `json:"signatureAlgorithm"`
	Signature string `json:"signature"`
}
type User struct {
	Type     string `json:"type"`
	IssuerSN uint64 `json:"issuerSn"`
	Subject  struct {
		Callsign  string `json:"callsign"`
		UID       uint64 `json:"uid"`
		PublicKey string `json:"publicKey"`
	} `json:"subject"`
	IAT         uint64 `json:"iat"`
	EXP         uint64 `json:"exp"`
	Algorithm   string `json:"signatureAlgorithm"`
	Signature   string `json:"signature"`
	Fingerprint string `json:"certFingerprint"`
}
type Bundle struct {
	Root         Root         `json:"rootCert"`
	Intermediate Intermediate `json:"intermediateCert"`
	User         User         `json:"userCert"`
}
type Bound struct {
	Bundle                  Bundle
	User                    protocol.UserCert
	RootFingerprint         string
	IntermediateFingerprint string
	Expires                 uint64
}

func (r Root) TBS() []byte {
	pub, _ := protocol.Decode64(r.Subject.PublicKey, 32)
	return protocol.Encode([]any{"FMO", 4, "rootCA", r.SN, r.Issuer.Name, r.Issuer.Email, r.Subject.Name, pub, true, 1, r.Extensions.CRL, r.Extensions.License, r.Extensions.KeyID, r.IAT, r.EXP})
}
func (i Intermediate) TBS() []byte {
	pub, _ := protocol.Decode64(i.Subject.PublicKey, 32)
	parent, _ := protocol.Decode64(i.Issuer.PublicKey, 32)
	countries := append([]string{}, i.Extensions.Countries...)
	sort.Strings(countries)
	a := make([]any, len(countries))
	for j, c := range countries {
		a[j] = c
	}
	return protocol.Encode([]any{"FMO", 4, "intermediateCA", i.SN, i.Issuer.SN, i.Issuer.Name, parent, i.Subject.Name, i.Subject.Email, pub, true, 0, i.Extensions.KeyID, i.Extensions.CRL, i.Extensions.License, i.Extensions.UIDRange.Start, i.Extensions.UIDRange.End, a, i.IAT, i.EXP})
}
func (u User) Cert() protocol.UserCert {
	pub, _ := protocol.Decode64(u.Subject.PublicKey, 32)
	sig, _ := protocol.Decode64(u.Signature, 64)
	return protocol.UserCert{IssuerSN: u.IssuerSN, Callsign: u.Subject.Callsign, UID: u.Subject.UID, PublicKey: pub, IAT: u.IAT, EXP: u.EXP, Signature: sig}
}
func verify(pub, tbs []byte, sig string) bool {
	b, e := protocol.Decode64(sig, 64)
	return e == nil && ValidPublicKey(pub) && ed25519.Verify(pub, tbs, b)
}
func valid(iat, exp, now uint64) bool {
	return iat < exp && exp <= protocol.MaxInteger && iat <= now && now < exp
}
func VerifyBundle(b Bundle, roots []string, now uint64, country string) (Bound, error) {
	fail := func() (Bound, error) {
		return Bound{}, errors.New("identity chain, trust, validity or authorization failed")
	}
	r, i, u := b.Root, b.Intermediate, b.User
	c := u.Cert()
	rf := protocol.Hash(r.TBS())
	trusted := false
	for _, pin := range roots {
		if pin == rf {
			trusted = true
		}
	}
	rp, re := protocol.Decode64(r.Subject.PublicKey, 32)
	ip, ie := protocol.Decode64(i.Subject.PublicKey, 32)
	_, ue := protocol.Decode64(u.Subject.PublicKey, 32)
	if !bundleShape(b) || !ValidPublicKey(c.PublicKey) || !trusted || re != nil || ie != nil || ue != nil || r.Type != "rootCA" || i.Type != "intermediateCA" || u.Type != "userCert" || r.Algorithm != "Ed25519" || i.Algorithm != "Ed25519" || u.Algorithm != "Ed25519" || !r.Extensions.IsCA || r.Extensions.PathLen != 1 || !i.Extensions.IsCA || i.Extensions.PathLen != 0 || !valid(r.IAT, r.EXP, now) || !valid(i.IAT, i.EXP, now) || !valid(u.IAT, u.EXP, now) || i.IAT < r.IAT || i.EXP > r.EXP || u.IAT < i.IAT || u.EXP > i.EXP || i.Issuer.SN != r.SN || i.Issuer.PublicKey != r.Subject.PublicKey || u.IssuerSN != i.SN || !protocol.ValidCallsign(c.Callsign) || c.UID > 0xffffffff || i.Extensions.UIDRange.Start > i.Extensions.UIDRange.End || c.UID < i.Extensions.UIDRange.Start || c.UID > i.Extensions.UIDRange.End || c.Fingerprint() != u.Fingerprint {
		return fail()
	}
	allowed := len(i.Extensions.Countries) == 0
	seen := map[string]bool{}
	for _, cc := range i.Extensions.Countries {
		if seen[cc] || len(cc) != 2 {
			return fail()
		}
		seen[cc] = true
		if cc == country {
			allowed = true
		}
	}
	if !allowed || !verify(rp, r.TBS(), r.Signature) || !verify(rp, i.TBS(), i.Signature) || !verify(ip, c.TBS(), u.Signature) {
		return fail()
	}
	return Bound{b, c, rf, protocol.Hash(i.TBS()), u.EXP}, nil
}

func bundleShape(b Bundle) bool {
	r, i, u := b.Root, b.Intermediate, b.User
	if r.SN == 0 || r.SN > protocol.MaxInteger || i.SN == 0 || i.SN > protocol.MaxInteger || u.IssuerSN == 0 || i.Extensions.UIDRange.End > 0xffffffff || len(i.Extensions.Countries) > 256 {
		return false
	}
	for _, v := range []struct {
		s        string
		min, max int
	}{
		{r.Issuer.Name, 1, 256}, {r.Issuer.Email, 0, 254}, {r.Subject.Name, 1, 256}, {r.Extensions.CRL, 0, 2048}, {r.Extensions.License, 0, 2048}, {r.Extensions.KeyID, 1, 128},
		{i.Issuer.Name, 1, 256}, {i.Subject.Name, 1, 256}, {i.Subject.Email, 0, 254}, {i.Extensions.CRL, 0, 2048}, {i.Extensions.License, 0, 2048}, {i.Extensions.KeyID, 1, 128},
	} {
		n := utf8.RuneCountInString(v.s)
		if !utf8.ValidString(v.s) || n < v.min || n > v.max {
			return false
		}
	}
	for _, cc := range i.Extensions.Countries {
		if len(cc) != 2 || strings.Trim(cc, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
			return false
		}
	}
	return true
}
