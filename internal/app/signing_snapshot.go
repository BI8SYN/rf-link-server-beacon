package app

import (
	"context"
	"errors"
	"github.com/BI8SYN/rf-link-server-beacon/internal/identity"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
)

func (s *Service) pendingIdentity() (signer.Identity, identity.Bound, error) {
	p := s.state.Pending
	if p == nil || p.Identity == nil || !signer.ValidID(p.BroadcastID) || p.BroadcastID != s.state.BroadcastID || p.Station == nil {
		return signer.Identity{}, identity.Bound{}, errors.New("pending signing request has no complete CERT snapshot; refusing to rebind it")
	}
	i := *p.Identity
	if i.BroadcastID != p.BroadcastID || i.ServerID != s.state.ServerID || i.EnrollmentID != s.state.EnrollmentID {
		return signer.Identity{}, identity.Bound{}, errors.New("pending certificate snapshot belongs to a different identity")
	}
	b, e := i.VerifyCertificate(s.cfg.Trust.Roots, s.now(), s.cfg.Callsign, p.Station.CountryCode)
	return i, b, e
}

func (s *Service) clearPending() error {
	candidate := s.state
	candidate.Pending = nil
	candidate.PendingSince = 0
	if e := s.store.Write(candidate); e != nil {
		return e
	}
	s.state = candidate
	return nil
}

// Both the currently authorized chain and the original signing certificate
// must remain unrevoked. Keep the most conservative CRL transmission deadline.
func (s *Service) checkPendingCRLs(ctx context.Context, b identity.Bound) error {
	freshDeadline := s.crlDeadline
	if e := s.checkCRLs(ctx, b); e != nil {
		return e
	}
	if !freshDeadline.IsZero() && freshDeadline.Before(s.crlDeadline) {
		s.crlDeadline = freshDeadline
	}
	return nil
}
