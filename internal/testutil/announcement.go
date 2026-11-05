package testutil

import (
	"crypto/ed25519"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"testing"
	"time"
)

func Announcement(t *testing.T, now time.Time) signer.Announcement {
	return AnnouncementForCallsign(t, now, "BI8SYN")
}
func AnnouncementForCallsign(t *testing.T, now time.Time, callsign string) signer.Announcement {
	t.Helper()
	n := uint64(now.Unix())
	f := Identity(t, n, "https://pki.example.invalid/root", "https://pki.example.invalid/inter")
	f.Bundle.User.Subject.Callsign = callsign
	f.Bundle.User.Signature = protocol.B64(ed25519.Sign(f.IntermediateKey, f.Bundle.User.Cert().TBS()))
	f.Bundle.User.Fingerprint = f.Bundle.User.Cert().Fingerprint()
	c := f.Bundle.User.Cert()
	const id = "95b976da-d670-44ec-bb72-b876ad05f134"
	s := protocol.Station{SSID: 15, Latitude: "2505.12N", Longitude: "10242.22E", SymbolTable: "/", SymbolCode: "I", CountryCode: "CN", Name: "Test", Host: "voice.example.com", Port: 1883, FilterKM: 5000}
	i := signer.Identity{ServerID: id, EnrollmentID: id, BroadcastID: id, Callsign: c.Callsign, UID: c.UID, Fingerprint: c.Fingerprint(), BlobHash: c.BlobHash(), Blob: protocol.B64(c.Blob()), Bundle: f.Bundle, SigningEnabled: true, ServerTime: n}
	b, e := i.Verify([]string{f.RootPin}, now, c.Callsign, s.CountryCode)
	if e != nil {
		t.Fatal(e)
	}
	request := signer.SignRequest{RequestID: id, Station: s, Online: 0, Peak: 0}
	sig := protocol.B64(ed25519.Sign(f.UserKey, protocol.StationTBS(c, s, 0, 0, n/600)))
	line, _ := protocol.Line(c, s, 0, 0, sig)
	r := signer.Signature{RequestID: id, BroadcastID: id, Fingerprint: c.Fingerprint(), BlobHash: c.BlobHash(), TimeSalt: n / 600, SignedAt: n, ServerTime: n, SendBefore: min(n+60, (n/600+1)*600, c.EXP), VerifyUntil: min((n/600+2)*600, c.EXP), Signature: sig, APRSLine: line}
	packet, e := signer.VerifySignature(i, b, request, r, now, 0)
	if e != nil {
		t.Fatal(e)
	}
	return packet
}
