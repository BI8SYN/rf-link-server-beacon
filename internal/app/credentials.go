package app

import (
	"context"
	"errors"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"time"
)

func (s *Service) ensureCredential(ctx context.Context, force bool) error {
	now := uint64(s.now().Unix())
	valid := s.state.Credential != nil && s.state.Credential.ExpiresAt > now
	if !force && valid && s.state.ClaimRequestID == "" && now < s.state.CredentialRenewAt {
		return nil
	}
	if s.applicationClient == nil {
		return errors.New("application receipt missing; cannot automatically recover legacy credentials")
	}
	if now < s.state.ClaimAfter {
		if valid && !force {
			return nil
		}
		return &RetryError{time.Duration(s.state.ClaimAfter-now) * time.Second, "credential_recovery_pending"}
	}
	if s.state.ClaimRequestID == "" {
		id, e := signer.NewID()
		if e != nil {
			return e
		}
		candidate := s.state
		candidate.ClaimRequestID = id
		if e = s.store.Write(candidate); e != nil {
			return e
		}
		s.state = candidate
	}
	out, e := s.applicationClient.Claim(ctx, s.state.EnrollmentID, s.state.ClaimRequestID)
	if e != nil {
		var api *signer.APIError
		if errors.As(e, &api) {
			if api.Code == "credential_delivery_expired" {
				candidate := s.state
				candidate.ClaimRequestID = ""
				candidate.ClaimAfter = 0
				if err := s.store.Write(candidate); err != nil {
					return err
				}
				s.state = candidate
				return &RetryError{max(time.Second, api.RetryAfter), api.Code}
			}
			if api.Code == "credential_recovery_pending" {
				wait := max(time.Second, api.RetryAfter)
				candidate := s.state
				candidate.ClaimAfter = now + uint64(wait/time.Second)
				if err := s.store.Write(candidate); err != nil {
					return err
				}
				s.state = candidate
				if valid && !force {
					return nil
				}
				return &RetryError{wait, api.Code}
			}
		}
		return e
	}
	if out.ServerID != s.state.ServerID || out.BroadcastID != s.state.BroadcastID || out.ExpiresAt <= now || out.ExpiresAt > now+90*86400+30 || out.RetryUntil <= now || out.RetryUntil > now+900+30 {
		return errors.New("credential delivery resource or lifetime invalid")
	}
	client, e := signer.New(s.cfg.Signer.BaseURL, out.MachineToken, s.cfg.Signer.AllowLoopbackHTTP)
	if e != nil {
		return e
	}
	candidate := s.state
	candidate.Credential = &out
	candidate.ClaimRequestID = ""
	candidate.ClaimAfter = 0
	lead := uint64(86400)
	if out.ExpiresAt-now <= 86400 {
		lead = max(uint64(30), (out.ExpiresAt-now)/10)
	}
	if lead >= out.ExpiresAt-now {
		candidate.CredentialRenewAt = now + 1
	} else {
		candidate.CredentialRenewAt = out.ExpiresAt - lead
	}
	if e = s.store.Write(candidate); e != nil {
		return e
	}
	s.state = candidate
	s.client = client
	return nil
}
func (s *Service) machineIdentity(ctx context.Context) (signer.Identity, error) {
	out, e := s.client.Identity(ctx, s.state.BroadcastID)
	var api *signer.APIError
	if errors.As(e, &api) && api.Status == 401 && api.Code == "credential_invalid" && s.applicationClient != nil {
		if e = s.ensureCredential(ctx, true); e != nil {
			return signer.Identity{}, e
		}
		return s.client.Identity(ctx, s.state.BroadcastID)
	}
	return out, e
}
func (s *Service) machineSignature(ctx context.Context, id string, request signer.SignRequest) (signer.Signature, error) {
	out, e := s.client.SignStation(ctx, id, request)
	var api *signer.APIError
	if errors.As(e, &api) && api.Status == 401 && api.Code == "credential_invalid" && s.applicationClient != nil {
		if e = s.ensureCredential(ctx, true); e != nil {
			return signer.Signature{}, e
		}
		return s.client.SignStation(ctx, id, request)
	}
	return out, e
}

// AwaitIdentity never connects to APRS; ready state automatically claims credentials.
func (s *Service) AwaitIdentity(ctx context.Context, report func(string)) (signer.Identity, error) {
	last := ""
	attempt := 0
	for {
		if s.state.EnrollmentID == "" {
			if _, e := s.Enroll(ctx); e != nil {
				return signer.Identity{}, e
			}
		}
		info, e := s.Check(ctx)
		if e == nil {
			return info, nil
		}
		var retry *RetryError
		wait := time.Duration(0)
		if errors.As(e, &retry) {
			if retry.Code != last {
				report(retry.Code)
				last = retry.Code
			}
			wait = retry.After
		} else {
			var temporary interface{ Temporary() bool }
			var api *signer.APIError
			if (errors.As(e, &temporary) && temporary.Temporary()) || (errors.As(e, &api) && api.Retryable()) {
				attempt++
				wait = Backoff(attempt)
				if api != nil {
					wait = max(wait, api.RetryAfter)
				}
			} else {
				return signer.Identity{}, e
			}
		}
		if e = Wait(ctx, wait); e != nil {
			return signer.Identity{}, e
		}
	}
}
