package identity

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"sort"
)

type Entry struct {
	SN          *uint64 `json:"sn,omitempty"`
	UID         *uint64 `json:"uid,omitempty"`
	Fingerprint string  `json:"certFingerprint"`
	RevokedAt   uint64  `json:"revokedAt"`
	Reason      uint64  `json:"reason"`
}
type CRL struct {
	Type       string  `json:"type"`
	IssuerSN   uint64  `json:"issuerSn"`
	Number     uint64  `json:"crlNumber"`
	ThisUpdate uint64  `json:"thisUpdate"`
	NextUpdate uint64  `json:"nextUpdate"`
	Entries    []Entry `json:"entries"`
	Algorithm  string  `json:"signatureAlgorithm"`
	Signature  string  `json:"signature"`
}
type Mark struct {
	Number     uint64 `json:"number"`
	ThisUpdate uint64 `json:"thisUpdate"`
	Digest     string `json:"digest"`
}

func (c CRL) TBS() []byte {
	a := make([][]any, 0, len(c.Entries))
	for _, e := range c.Entries {
		n := uint64(0)
		if c.Type == "rootCRL" && e.SN != nil {
			n = *e.SN
		}
		if c.Type == "intermediateCRL" && e.UID != nil {
			n = *e.UID
		}
		hash, _ := protocol.Decode64(e.Fingerprint, 32)
		a = append(a, []any{n, hash, e.RevokedAt, e.Reason})
	}
	sort.Slice(a, func(i, j int) bool {
		if a[i][0].(uint64) != a[j][0].(uint64) {
			return a[i][0].(uint64) < a[j][0].(uint64)
		}
		return bytes.Compare(a[i][1].([]byte), a[j][1].([]byte)) < 0
	})
	entries := make([]any, len(a))
	for i, e := range a {
		entries[i] = e
	}
	return protocol.Encode([]any{"FMO", 4, c.Type, c.IssuerSN, c.Number, c.ThisUpdate, c.NextUpdate, entries})
}
func CheckCRL(c CRL, pub ed25519.PublicKey, issuer, iat, exp, uidStart, uidEnd, target uint64, fingerprint string, now uint64, previous Mark) (Mark, error) {
	fail := func() (Mark, error) {
		return Mark{}, errors.New("CRL invalid, expired, rolled back, conflicting or certificate revoked")
	}
	if (c.Type != "rootCRL" && c.Type != "intermediateCRL") || c.Algorithm != "Ed25519" || c.IssuerSN != issuer || !valid(c.ThisUpdate, c.NextUpdate, now) || c.ThisUpdate < iat || c.NextUpdate > exp || c.Number == 0 || c.IssuerSN == 0 || c.Number > protocol.MaxInteger || len(c.Entries) > 10000 {
		return fail()
	}
	seen := map[string]bool{}
	for _, e := range c.Entries {
		_, err := protocol.Decode64(e.Fingerprint, 32)
		if err != nil || seen[e.Fingerprint] || e.RevokedAt > c.ThisUpdate || e.Reason > 4 {
			return fail()
		}
		seen[e.Fingerprint] = true
		var id uint64
		if c.Type == "rootCRL" {
			if e.SN == nil || e.UID != nil || *e.SN == 0 || *e.SN > protocol.MaxInteger {
				return fail()
			}
			id = *e.SN
		} else {
			if e.UID == nil || e.SN != nil || *e.UID < uidStart || *e.UID > uidEnd {
				return fail()
			}
			id = *e.UID
		}
		if id == target && e.Fingerprint == fingerprint {
			return fail()
		}
	}
	tbs := c.TBS()
	if !verify(pub, tbs, c.Signature) {
		return fail()
	}
	mark := Mark{c.Number, c.ThisUpdate, protocol.Hash(tbs)}
	if previous.Digest != "" && (mark.Number < previous.Number || mark.ThisUpdate < previous.ThisUpdate || (mark.Number == previous.Number && mark.Digest != previous.Digest) || (mark.ThisUpdate == previous.ThisUpdate && mark.Digest != previous.Digest)) {
		return fail()
	}
	return mark, nil
}
