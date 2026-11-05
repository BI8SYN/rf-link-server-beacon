package identity_test

import (
	"crypto/ed25519"
	"github.com/BI8SYN/rf-link-server-beacon/internal/identity"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"github.com/BI8SYN/rf-link-server-beacon/internal/testutil"
	"testing"
)

func TestRejectCorrectlySignedInvalidUIDRange(t *testing.T) {
	f := testutil.Identity(t, 2000, "https://pki.example.invalid/root", "https://pki.example.invalid/inter")
	f.Bundle.Intermediate.Extensions.UIDRange.End = 1 << 32
	f.Bundle.Intermediate.Signature = protocol.B64(ed25519.Sign(f.RootKey, f.Bundle.Intermediate.TBS()))
	if _, e := identity.VerifyBundle(f.Bundle, []string{f.RootPin}, 2000, "CN"); e == nil {
		t.Fatal("out-of-schema UID range accepted")
	}
}

func TestEmptyCountryListMeansUnrestrictedButNonemptyListRestricts(t *testing.T) {
	f := testutil.Identity(t, 2000, "https://pki.example.invalid/root", "https://pki.example.invalid/inter")
	f.Bundle.Intermediate.Extensions.Countries = []string{}
	f.Bundle.Intermediate.Signature = protocol.B64(ed25519.Sign(f.RootKey, f.Bundle.Intermediate.TBS()))
	if _, e := identity.VerifyBundle(f.Bundle, []string{f.RootPin}, 2000, "CN"); e != nil {
		t.Fatal("empty country list must follow the explicit unrestricted policy", e)
	}
	f.Bundle.Intermediate.Extensions.Countries = []string{"US"}
	f.Bundle.Intermediate.Signature = protocol.B64(ed25519.Sign(f.RootKey, f.Bundle.Intermediate.TBS()))
	if _, e := identity.VerifyBundle(f.Bundle, []string{f.RootPin}, 2000, "CN"); e == nil {
		t.Fatal("nonempty country boundary bypassed")
	}
}
