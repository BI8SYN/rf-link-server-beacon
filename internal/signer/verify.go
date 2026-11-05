package signer

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/identity"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"time"
)

func (i Identity) Verify(roots []string, now time.Time, callsign, country string) (identity.Bound, error) {
	if !i.SigningEnabled || clockDifference(i.ServerTime, now) > 30 {
		return identity.Bound{}, errors.New("identity disabled or does not match configured clock")
	}
	return i.VerifyCertificate(roots, now, callsign, country)
}

// VerifyCertificate revalidates persisted public certificate material at the
// current time. It does not treat cached authorization flags or serverTime as
// current authority; callers must separately obtain fresh authorization/CRLs.
func (i Identity) VerifyCertificate(roots []string, now time.Time, callsign, country string) (identity.Bound, error) {
	var empty identity.Bound
	if !ValidID(i.ServerID) || !ValidID(i.EnrollmentID) || !ValidID(i.BroadcastID) || i.Callsign != callsign {
		return empty, errors.New("certificate identity ownership invalid")
	}
	b, e := identity.VerifyBundle(i.Bundle, roots, uint64(now.Unix()), country)
	if e != nil {
		return empty, e
	}
	blob, e := protocol.ParseBlob(i.Blob)
	if e != nil || !bytes.Equal(blob.Blob(), b.User.Blob()) || i.UID != b.User.UID || i.Callsign != b.User.Callsign || i.Fingerprint != b.User.Fingerprint() || i.BlobHash != b.User.BlobHash() {
		return empty, errors.New("identity CERT or fingerprint binding failed")
	}
	return b, nil
}
func clockDifference(server uint64, now time.Time) uint64 {
	n := uint64(now.Unix())
	if server > n {
		return server - n
	}
	return n - server
}

// Announcement can only be constructed after protocol and response verification.
type Announcement struct {
	line     string
	deadline time.Time
	source   string
}

func (a Announcement) Line() string        { return a.line }
func (a Announcement) Deadline() time.Time { return a.deadline }
func (a Announcement) Source() string      { return a.source }
func VerifySignature(i Identity, b identity.Bound, request SignRequest, r Signature, received time.Time, roundTrip time.Duration) (Announcement, error) {
	fail := func() (Announcement, error) {
		return Announcement{}, errors.New("signature response failed identity, payload or freshness verification")
	}
	if request.Station.Validate() != nil || request.Online > request.Peak || request.Peak > protocol.MaxInteger || !i.SigningEnabled || r.RequestID != request.RequestID || r.BroadcastID != i.BroadcastID || r.Fingerprint != b.User.Fingerprint() || r.BlobHash != b.User.BlobHash() || r.TimeSalt != r.SignedAt/600 || r.ServerTime < r.SignedAt || clockDifference(r.ServerTime, received) > 30 || uint64(received.Unix())/600 != r.TimeSalt || r.SignedAt >= b.Expires {
		return fail()
	}
	// A recovered snapshot may have a different country from the current config.
	if _, e := identity.VerifyBundle(b.Bundle, []string{b.RootFingerprint}, uint64(received.Unix()), request.Station.CountryCode); e != nil {
		return fail()
	}
	sendBefore := min(r.SignedAt+60, (r.TimeSalt+1)*600, b.Expires)
	until := min((r.TimeSalt+2)*600, b.Expires)
	if r.SendBefore != sendBefore || r.VerifyUntil != until || r.ServerTime >= r.SendBefore || roundTrip < 0 || roundTrip > 10*time.Second {
		return fail()
	}
	sig, e := protocol.Decode64(r.Signature, 64)
	if e != nil || !ed25519.Verify(b.User.PublicKey, protocol.StationTBS(b.User, request.Station, request.Online, request.Peak, r.TimeSalt), sig) {
		return fail()
	}
	line, e := protocol.Line(b.User, request.Station, request.Online, request.Peak, r.Signature)
	if e != nil {
		return fail()
	}
	remaining := time.Duration(r.SendBefore-r.ServerTime)*time.Second - roundTrip - 5*time.Second
	if remaining <= 0 {
		return fail()
	}
	source := b.User.Callsign
	if request.Station.SSID != 0 {
		source += fmt.Sprintf("-%d", request.Station.SSID)
	}
	// SIG authenticates the canonical CBOR snapshot, not server-supplied text.
	// Only emit our bounded local encoding of that verified snapshot. APRSLine
	// is upstream diagnostic text and never enters the send path.
	return Announcement{line, received.Add(remaining), source}, nil
}

// LimitDeadline additionally bounds publication by locally verified CRL freshness.
func (a Announcement) LimitDeadline(deadline time.Time) Announcement {
	if deadline.Before(a.deadline) {
		a.deadline = deadline
	}
	return a
}
