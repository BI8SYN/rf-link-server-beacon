package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
)

// Progress exposes only operator-facing fields, never the application receipt
// or machine credential. The managed supervisor persists it separately.
type Progress struct {
	Phase           string `json:"phase"`
	EnrollmentID    string `json:"enrollmentId,omitempty"`
	UID             uint64 `json:"uid,omitempty"`
	LastPublished   uint64 `json:"lastPublished,omitempty"`
	NextBroadcastAt uint64 `json:"nextBroadcastAt,omitempty"`
	RetryAt         int64  `json:"retryAt,omitempty"`
	Reason          string `json:"reason,omitempty"`
	UpdatedAt       int64  `json:"updatedAt"`
}

type ManagedSAS interface {
	Configure([]byte) error
	Ready() bool
	Disable()
}

// Manage owns only a new, dedicated stack. Authorization is rechecked
// periodically even when the next scheduled transmission is minutes away.
func (s *Service) Manage(ctx context.Context, sas ManagedSAS, report func(Progress) error) error {
	defer sas.Disable()
	const refresh = 15 * time.Second
	progress := Progress{EnrollmentID: s.state.EnrollmentID, LastPublished: s.state.LastPublished}
	update := func(phase, reason string, delay time.Duration) error {
		progress.Phase = phase
		progress.Reason = reason
		progress.UpdatedAt = time.Now().Unix()
		progress.NextBroadcastAt = 0
		if (phase == "broadcasting" || phase == "broadcast_retry") && s.state.LastAttempt != 0 {
			progress.NextBroadcastAt = s.state.LastAttempt + 600
		}
		progress.RetryAt = 0
		if delay > 0 && phase != "broadcasting" {
			progress.RetryAt = time.Now().Add(delay).Unix()
		}
		return report(progress)
	}
	failures := 0
	nextCycle := time.Time{}
	for {
		delay, phase, reason := refresh, "broadcasting", ""
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.state.EnrollmentID == "" {
			if e := update("submitting_application", "", 0); e != nil {
				return e
			}
			if _, e := s.Enroll(ctx); e != nil {
				d, ok := retryDelay(e, failures)
				if !ok {
					return e
				}
				failures++
				if e = update("retrying", e.Error(), d); e != nil {
					return e
				}
				if e = Wait(ctx, d); e != nil {
					return e
				}
				continue
			}
			progress.EnrollmentID = s.state.EnrollmentID
		}
		info, b, e := s.check(ctx)
		if e != nil {
			sas.Disable()
			d, ok := retryDelay(e, failures)
			if !ok {
				return e
			}
			failures++
			phase, reason, delay = "retrying", e.Error(), d
			var re *RetryError
			if errors.As(e, &re) {
				switch re.Code {
				case "pending_review", "awaiting_signature", "publishing":
					phase, reason = re.Code, ""
				}
			}
		} else {
			progress.UID = info.UID
			raw, e := json.MarshalIndent(s.sasConfig(b, "/home/app/.sas/roots", "0.0.0.0"), "", "  ")
			if e != nil {
				return e
			}
			if e = sas.Configure(raw); e != nil {
				return e
			}
			if !sas.Ready() {
				phase, delay = "waiting_mqtt_authentication", 3*time.Second
				reason = "SAS configuration or MQTT authentication has not been confirmed"
			} else {
				if s.state.Fingerprint != info.Fingerprint {
					s.state.Fingerprint = info.Fingerprint
					if e = s.store.Write(s.state); e != nil {
						return e
					}
				}
				if time.Now().Before(nextCycle) {
					delay = min(refresh, time.Until(nextCycle))
				} else {
					if s.state.LastPublished == 0 {
						if e = update("sending_first_announcement", "", 0); e != nil {
							return e
						}
					}
					result, e := s.cycle(ctx, true, info, b, sas.Ready)
					if e != nil {
						d, ok := retryDelay(e, failures)
						if !ok {
							return e
						}
						failures++
						nextCycle = time.Now().Add(d)
						delay = min(refresh, d)
						if re, ok := e.(*RetryError); !ok || re.Code != "broadcast interval not elapsed" {
							phase, reason = "broadcast_retry", e.Error()
						}
					} else if result.Published {
						failures = 0
						progress.LastPublished = s.state.LastPublished
						nextCycle = time.Now().Add(600 * time.Second)
					}
				}
			}
		}
		if e := update(phase, reason, delay); e != nil {
			return e
		}
		if e := Wait(ctx, delay); e != nil {
			return e
		}
	}
}

func retryDelay(e error, failures int) (time.Duration, bool) {
	var retry *RetryError
	if errors.As(e, &retry) {
		return max(time.Second, retry.After), true
	}
	var api *signer.APIError
	if errors.As(e, &api) && api.Retryable() {
		return max(Backoff(failures+1), api.RetryAfter), true
	}
	var temporary interface{ Temporary() bool }
	if errors.As(e, &temporary) && temporary.Temporary() {
		return Backoff(failures + 1), true
	}
	return 0, false
}
