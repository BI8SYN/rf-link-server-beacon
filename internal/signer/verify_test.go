package signer_test

import (
	"crypto/ed25519"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"github.com/BI8SYN/rf-link-server-beacon/internal/testutil"
	"strings"
	"testing"
	"time"
)

func TestResponseRequiresTrustedIdentitySignedSnapshotAndFreshDeadline(t *testing.T) {
	now := time.Unix(1793318410, 0)
	f := testutil.Identity(t, uint64(now.Unix()), "https://pki.example.invalid/root", "https://pki.example.invalid/inter")
	cert := f.Bundle.User.Cert()
	const id = "95b976da-d670-44ec-bb72-b876ad05f134"
	station := protocol.Station{SSID: 15, Latitude: "2505.12N", Longitude: "10242.22E", SymbolTable: "/", SymbolCode: "I", CountryCode: "CN", Name: "测试", Host: "voice.example.com", Port: 1883, FilterKM: 5000}
	info := signer.Identity{ServerID: id, EnrollmentID: id, BroadcastID: id, Callsign: "BI8SYN", UID: 20, Fingerprint: cert.Fingerprint(), BlobHash: cert.BlobHash(), Blob: protocol.B64(cert.Blob()), Bundle: f.Bundle, SigningEnabled: true, ServerTime: uint64(now.Unix())}
	bound, e := info.Verify([]string{f.RootPin}, now, "BI8SYN", station.CountryCode)
	if e != nil {
		t.Fatal(e)
	}
	req := signer.SignRequest{RequestID: id, Station: station, Online: 2, Peak: 6}
	n := uint64(now.Unix())
	sig := protocol.B64(ed25519.Sign(f.UserKey, protocol.StationTBS(cert, station, 2, 6, n/600)))
	line, _ := protocol.Line(cert, station, 2, 6, sig)
	resp := signer.Signature{RequestID: id, BroadcastID: id, Fingerprint: cert.Fingerprint(), BlobHash: cert.BlobHash(), TimeSalt: n / 600, SignedAt: n, ServerTime: n, SendBefore: n + 60, VerifyUntil: (n/600 + 2) * 600, Signature: sig, APRSLine: line}
	packet, e := signer.VerifySignature(info, bound, req, resp, now, time.Second)
	if e != nil || packet.Line() != line || !packet.Deadline().After(now) {
		t.Fatal(e)
	}
	for _, hint := range []string{"", strings.Replace(line, "IFMO-V4,", "I FMO-V4,", 1), "OTHER>APRS:wrong endpoint", line + "\r\nINJECT>APRS:bad"} {
		r := resp
		r.APRSLine = hint
		p, err := signer.VerifySignature(info, bound, req, r, now, time.Second)
		if err != nil || p.Line() != line || strings.ContainsAny(p.Line(), "\r\n") || !strings.Contains(p.Line(), "IFMO-V4,STATION,") {
			t.Errorf("server text influenced locally constructed signed packet: %v", err)
		}
	}
	for _, name := range []string{"signature", "fingerprint", "request", "clock", "expired", "deadline", "window", "station", "counts", "country"} {
		t.Run(name, func(t *testing.T) {
			r := resp
			request := req
			at := now
			switch name {
			case "signature":
				r.Signature = protocol.B64(make([]byte, 64))
			case "fingerprint":
				r.Fingerprint = protocol.Hash([]byte("wrong"))
			case "request":
				r.RequestID = "other"
			case "clock":
				at = now.Add(time.Minute)
			case "expired":
				r.ServerTime = r.SendBefore
			case "deadline":
				r.SendBefore++
			case "window":
				r.TimeSalt++
			case "station":
				request.Station.Host = "other.example.com"
			case "country":
				request.Station.CountryCode = "US"
			case "counts":
				request.Online = 3
			}
			if _, e := signer.VerifySignature(info, bound, request, r, at, time.Second); e == nil {
				t.Fatal("untrusted response accepted")
			}
		})
	}
	t.Run("valid signature cannot bypass certificate country scope", func(t *testing.T) {
		request := req
		request.Station.CountryCode = "US"
		r := resp
		r.Signature = protocol.B64(ed25519.Sign(f.UserKey, protocol.StationTBS(cert, request.Station, request.Online, request.Peak, r.TimeSalt)))
		r.APRSLine, _ = protocol.Line(cert, request.Station, request.Online, request.Peak, r.Signature)
		if _, e := signer.VerifySignature(info, bound, request, r, now, time.Second); e == nil {
			t.Fatal("valid signature bypassed certificate country scope")
		}
	})
}

func TestAnnouncementDeadlineCanOnlyBeShortened(t *testing.T) {
	now := time.Unix(1793318410, 0)
	packet := testutil.Announcement(t, now)
	original := packet.Deadline()
	if packet.LimitDeadline(original.Add(time.Hour)).Deadline() != original {
		t.Fatal("deadline extended")
	}
	earlier := now.Add(3 * time.Second)
	if packet.LimitDeadline(earlier).Deadline() != earlier {
		t.Fatal("CRL deadline not applied")
	}
}
