package identity

import (
	"bytes"
	"encoding/json"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"os"
	"testing"
)

func fixture(t *testing.T) (Bundle, string) {
	t.Helper()
	raw, e := os.ReadFile("../protocol/testdata/identity-bundle.json")
	if e != nil {
		t.Fatal(e)
	}
	var v struct {
		Bundle Bundle `json:"bundle"`
		Root   string `json:"trustedRootFingerprint"`
	}
	if e = json.Unmarshal(raw, &v); e != nil {
		t.Fatal(e)
	}
	return v.Bundle, v.Root
}
func TestPublicBundleTrustAndBinding(t *testing.T) {
	b, root := fixture(t)
	bound, e := VerifyBundle(b, []string{root}, 1000, "CN")
	if e != nil || bound.User.UID != 20 {
		t.Fatalf("valid chain: %v", e)
	}
	for _, which := range []string{"root", "expired", "uid", "callsign", "country", "issuer", "signature"} {
		t.Run(which, func(t *testing.T) {
			x, r := fixture(t)
			now := uint64(1000)
			country := "CN"
			switch which {
			case "root":
				r = protocol.Hash([]byte("wrong"))
			case "expired":
				now = 8000
			case "uid":
				x.User.Subject.UID = 101
			case "callsign":
				x.User.Subject.Callsign = "OTHER"
			case "country":
				country = "JP"
			case "issuer":
				x.Intermediate.Issuer.SN++
			case "signature":
				x.Root.Signature = protocol.B64(make([]byte, 64))
			}
			if _, e := VerifyBundle(x, []string{r}, now, country); e == nil {
				t.Fatal("invalid chain accepted")
			}
		})
	}
}

func TestRejectWeakAndNoncanonicalPublicKeys(t *testing.T) {
	for _, raw := range [][]byte{make([]byte, 32), append([]byte{1}, make([]byte, 31)...), append([]byte{0xed}, append(bytes.Repeat([]byte{0xff}, 30), 0x7f)...)} {
		if ValidPublicKey(raw) {
			t.Fatalf("weak or noncanonical public key accepted: %x", raw)
		}
	}
	b, _ := fixture(t)
	pub, _ := protocol.Decode64(b.User.Subject.PublicKey, 32)
	if !ValidPublicKey(pub) {
		t.Fatal("valid fixture key rejected")
	}
}
