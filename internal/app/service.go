package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/aprs"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"github.com/BI8SYN/rf-link-server-beacon/internal/identity"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"os"
	"path/filepath"
	"time"
)

// SigningSnapshot can read the old pending record, but legacy fields never
// enter SignRequest. Such records are discarded durably before any HTTP call.
type SigningSnapshot struct {
	RequestID            string            `json:"requestId"`
	BroadcastID          string            `json:"broadcastId,omitempty"`
	Identity             *signer.Identity  `json:"identity,omitempty"`
	Station              *protocol.Station `json:"station,omitempty"`
	Online               uint64            `json:"online"`
	Peak                 uint64            `json:"peak"`
	LegacyProfileVersion *uint64           `json:"expectedProfileVersion,omitempty"`
}

type State struct {
	ApplicationAccessToken string                   `json:"applicationAccessToken,omitempty"`
	ClaimRequestID         string                   `json:"claimRequestId,omitempty"`
	ClaimAfter             uint64                   `json:"claimAfter,omitempty"`
	Credential             *signer.Credential       `json:"credential,omitempty"`
	CredentialRenewAt      uint64                   `json:"credentialRenewAt,omitempty"`
	Schema                 uint64                   `json:"schema"`
	Origin                 string                   `json:"origin"`
	Callsign               string                   `json:"callsign"`
	ServerID               string                   `json:"serverId"`
	EnrollmentID           string                   `json:"enrollmentId"`
	BroadcastID            string                   `json:"broadcastId"`
	Enrollment             *signer.Enrollment       `json:"enrollment,omitempty"`
	Pending                *SigningSnapshot         `json:"pending,omitempty"`
	PendingSince           uint64                   `json:"pendingSince"`
	Fingerprint            string                   `json:"fingerprint"`
	ProfileVersion         uint64                   `json:"profileVersion,omitempty"` // Legacy disk metadata only.
	LastAttempt            uint64                   `json:"lastAttempt"`
	LastPublished          uint64                   `json:"lastPublished"`
	CRLs                   map[string]identity.Mark `json:"crls"`
}
type Service struct {
	cfg               config.Config
	store             *Store
	client            *signer.Client
	applicationClient *signer.Client
	state             State
	now               func() time.Time
	recovered         bool
	crlDeadline       time.Time
}
type RetryError struct {
	After time.Duration
	Code  string
}

func (e *RetryError) Error() string { return fmt.Sprintf("%s; retry after %s", e.Code, e.After) }
func NewService(cfg config.Config) (*Service, error) {
	if e := cfg.Validate(); e != nil {
		return nil, e
	}
	store, e := OpenStore(cfg.StateDir)
	if e != nil {
		return nil, e
	}
	s := &Service{cfg: cfg, store: store, now: time.Now}
	e = store.Read(&s.state)
	if errors.Is(e, os.ErrNotExist) {
		s.state = State{Schema: 1, Origin: cfg.Signer.BaseURL, Callsign: cfg.Callsign, CRLs: map[string]identity.Mark{}}
		e = store.Write(s.state)
	}
	if e == nil && (s.state.Schema != 1 || s.state.Origin != cfg.Signer.BaseURL || s.state.Callsign != cfg.Callsign) {
		e = errors.New("state belongs to another signer or callsign")
	}
	if e != nil {
		store.Close()
		return nil, e
	}
	if s.state.CRLs == nil {
		s.state.CRLs = map[string]identity.Mark{}
	}
	if s.state.Pending != nil && (s.state.Pending.Station == nil || s.state.Pending.LegacyProfileVersion != nil) {
		s.state.Pending = nil
		s.state.PendingSince = 0
		if e = store.Write(s.state); e != nil {
			store.Close()
			return nil, e
		}
	}
	if s.state.ApplicationAccessToken != "" {
		if s.state.Enrollment == nil {
			store.Close()
			return nil, errors.New("application receipt has no durable request")
		}
		s.applicationClient, e = signer.NewApplication(cfg.Signer.BaseURL, s.state.ApplicationAccessToken, cfg.Signer.AllowLoopbackHTTP)
		if e != nil {
			store.Close()
			return nil, e
		}
	}
	if s.state.Credential != nil {
		s.client, e = signer.New(cfg.Signer.BaseURL, s.state.Credential.MachineToken, cfg.Signer.AllowLoopbackHTTP)
		if e != nil {
			store.Close()
			return nil, e
		}
	} else if s.applicationClient == nil && s.state.EnrollmentID != "" {
		token, err := config.Secret(cfg.Signer.CredentialFile)
		if err != nil {
			store.Close()
			return nil, errors.New("legacy identity has no usable credential or application receipt")
		}
		s.client, e = signer.New(cfg.Signer.BaseURL, token, cfg.Signer.AllowLoopbackHTTP)
		if e != nil {
			store.Close()
			return nil, e
		}
	}
	s.recovered = s.state.Pending != nil
	return s, nil
}
func (s *Service) Close() error { return s.store.Close() }
func (s *Service) Enroll(ctx context.Context) (signer.Created, error) {
	if s.state.EnrollmentID != "" {
		return signer.Created{ServerID: s.state.ServerID, EnrollmentID: s.state.EnrollmentID, BroadcastID: s.state.BroadcastID, State: "existing"}, nil
	}
	if s.state.Enrollment == nil {
		if s.cfg.Applicant == nil || s.cfg.Applicant.Validate() != nil {
			return signer.Created{}, errors.New("applicant name, email and authorization note required; update private configuration")
		}
		id, e := signer.NewID()
		if e != nil {
			return signer.Created{}, e
		}
		token, e := signer.NewApplicationToken(id)
		if e != nil {
			return signer.Created{}, e
		}
		applicant := *s.cfg.Applicant
		candidate := s.state
		candidate.Enrollment = &signer.Enrollment{RequestID: id, Callsign: s.cfg.Callsign, Label: s.cfg.Label, PurposeNote: s.cfg.PurposeNote, Applicant: &applicant}
		candidate.ApplicationAccessToken = token
		if e = s.store.Write(candidate); e != nil {
			return signer.Created{}, e
		}
		s.state = candidate
	}
	if s.state.ApplicationAccessToken == "" {
		return signer.Created{}, errors.New("legacy application has no receipt; it cannot be claimed by public ID")
	}
	client, e := signer.NewApplication(s.cfg.Signer.BaseURL, s.state.ApplicationAccessToken, s.cfg.Signer.AllowLoopbackHTTP)
	if e != nil {
		return signer.Created{}, e
	}
	s.applicationClient = client
	out, e := s.applicationClient.Enroll(ctx, *s.state.Enrollment)
	if e != nil {
		return out, e
	}
	s.state.ServerID = out.ServerID
	s.state.EnrollmentID = out.EnrollmentID
	s.state.BroadcastID = out.BroadcastID
	return out, s.store.Write(s.state)
}
func (s *Service) Status(ctx context.Context) (signer.Status, error) {
	if !signer.ValidID(s.state.EnrollmentID) {
		return signer.Status{}, errors.New("no enrollment; run enroll first")
	}
	if s.applicationClient == nil {
		return signer.Status{}, errors.New("legacy identity has no application receipt for status queries")
	}
	out, e := s.applicationClient.Poll(ctx, s.state.EnrollmentID)
	if e == nil && (out.ServerID != s.state.ServerID || out.BroadcastID != s.state.BroadcastID) {
		e = errors.New("enrollment belongs to a different registered identity")
	}
	return out, e
}
func (s *Service) Check(ctx context.Context) (signer.Identity, error) {
	out, _, e := s.check(ctx)
	return out, e
}
func (s *Service) check(ctx context.Context) (signer.Identity, identity.Bound, error) {
	var empty signer.Identity
	var bound identity.Bound
	var status signer.Status
	var e error
	if s.applicationClient != nil {
		status, e = s.Status(ctx)
		if e != nil {
			return empty, bound, e
		}
		if status.State != "ready" {
			if status.State == "pending_review" || status.State == "awaiting_signature" || status.State == "publishing" {
				return empty, bound, &RetryError{max(30*time.Second, time.Duration(status.PollAfter)*time.Second), status.State}
			}
			return empty, bound, errors.New("identity is not ready: " + status.State)
		}
		if e = s.ensureCredential(ctx, false); e != nil {
			return empty, bound, e
		}
	}
	if s.client == nil {
		return empty, bound, errors.New("no machine credential; submit an application and wait for approval")
	}
	info, e := s.machineIdentity(ctx)
	if e != nil {
		return empty, bound, e
	}
	if info.ServerID != s.state.ServerID || info.EnrollmentID != s.state.EnrollmentID {
		return empty, bound, errors.New("identity ownership differs from saved application")
	}
	if s.applicationClient != nil && (status.Fingerprint == nil || *status.Fingerprint != info.Fingerprint) {
		return empty, bound, &RetryError{time.Second, "identity changed while refreshing; refresh again"}
	}
	bound, e = info.Verify(s.cfg.Trust.Roots, s.now(), s.cfg.Callsign, s.cfg.Station.CountryCode)
	if e != nil {
		return empty, bound, e
	}
	if s.state.Credential != nil && s.state.Credential.ExpiresAt > bound.Expires {
		return empty, bound, errors.New("machine credential lifetime exceeds verified certificate chain")
	}
	if e = s.checkCRLs(ctx, bound); e != nil {
		return empty, bound, e
	}
	return info, bound, nil
}
func (s *Service) checkCRLs(ctx context.Context, b identity.Bound) error {
	s.crlDeadline = time.Time{}
	r, i := b.Bundle.Root, b.Bundle.Intermediate
	items := []struct {
		url, kind, pub, pin, targetFingerprint string
		issuer, iat, exp, start, end, target   uint64
	}{{r.Extensions.CRL, "rootCRL", r.Subject.PublicKey, b.RootFingerprint, b.IntermediateFingerprint, r.SN, r.IAT, r.EXP, 0, protocol.MaxInteger, i.SN}, {i.Extensions.CRL, "intermediateCRL", i.Subject.PublicKey, b.IntermediateFingerprint, b.User.Fingerprint(), i.SN, i.IAT, i.EXP, i.Extensions.UIDRange.Start, i.Extensions.UIDRange.End, b.User.UID}}
	for _, item := range items {
		if !s.cfg.AllowCRL(item.url) {
			return errors.New("certificate CRL URL is not in the configured allowlist")
		}
		var crl identity.CRL
		if e := s.client.PublicJSON(ctx, item.url, &crl); e != nil {
			return e
		}
		if crl.Type != item.kind {
			return errors.New("CRL type does not match issuer")
		}
		key := item.pin + "/" + item.kind
		pub, e := protocol.Decode64(item.pub, 32)
		if e != nil {
			return e
		}
		mark, e := identity.CheckCRL(crl, pub, item.issuer, item.iat, item.exp, item.start, item.end, item.target, item.targetFingerprint, uint64(s.now().Unix()), s.state.CRLs[key])
		if e != nil {
			return e
		}
		if crl.NextUpdate <= uint64(s.now().Unix())+75 {
			return errors.New("CRL expires too soon to publish a fresh announcement")
		}
		deadline := time.Unix(int64(crl.NextUpdate), 0).Add(-5 * time.Second)
		if s.crlDeadline.IsZero() || deadline.Before(s.crlDeadline) {
			s.crlDeadline = deadline
		}
		s.state.CRLs[key] = mark
		// A read-only SAS preview verifies the same CRLs but must not mutate
		// the live daemon's state or compete for its instance lock.
		if s.store != nil {
			if e = s.store.Write(s.state); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *Service) sasConfig(b identity.Bound, roots, addr string) map[string]any {
	payload := map[string]any{"Server": map[string]any{"Uid": b.User.UID, "Callsign": b.User.Callsign, "IssuerSn": b.User.IssuerSN, "CertFingerprint": b.User.Fingerprint(), "Admins": []any{}}, "Mqtt": map[string]any{"Host": s.cfg.Station.Host, "Port": s.cfg.Station.Port, "Clusters": []any{}}, "Trust": map[string]any{"AllowIssuerSn": []uint64{}, "RootsDir": roots}, "Crl": map[string]any{"RefreshSec": 14400}, "Http": map[string]any{"Addr": addr, "Port": 8080, "TtlSec": 14400, "ResponseTemplate": "emqx", "MaxBodyBytes": 65536, "MaxConcurrent": 128}, "Log": map[string]any{"Level": "Info"}, "Update": map[string]any{"Enabled": false}}
	return payload
}

func (s *Service) ExportSAS(ctx context.Context, path string) error {
	_, b, e := s.check(ctx)
	if e != nil {
		return e
	}
	payload := s.sasConfig(b, "/var/lib/fmo-sas/.sas/roots", "127.0.0.1")
	raw, e := json.MarshalIndent(payload, "", "  ")
	if e != nil {
		return e
	}
	if !filepath.IsAbs(path) {
		return errors.New("SAS output must be an absolute new file path")
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return errors.New("cannot create SAS output; existing files are never overwritten")
	}
	if _, e = f.Write(append(raw, '\n')); e != nil {
		f.Close()
		return e
	}
	e = f.Sync()
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	s.state.Fingerprint = b.User.Fingerprint()
	return s.store.Write(s.state)
}

type CycleResult struct {
	Verified    bool   `json:"verified"`
	Published   bool   `json:"published"`
	Fingerprint string `json:"certificateFingerprint"`
}

func (s *Service) Cycle(ctx context.Context, send bool) (CycleResult, error) {
	var result CycleResult
	info, b, e := s.check(ctx)
	if e != nil {
		return result, e
	}
	return s.cycle(ctx, send, info, b, nil)
}

func (s *Service) cycle(ctx context.Context, send bool, info signer.Identity, b identity.Bound, ready func() bool) (CycleResult, error) {
	var result CycleResult
	var e error

	if send && s.state.Fingerprint != b.User.Fingerprint() {
		return result, errors.New("SAS identity not synchronized; run sync-identity and apply generated configuration")
	}
	now := s.now()
	if send && s.state.LastAttempt != 0 {
		next := time.Unix(int64(s.state.LastAttempt), 0).Add(600 * time.Second)
		if now.Before(next) {
			return result, &RetryError{next.Sub(now), "broadcast interval not elapsed"}
		}
	}
	if s.state.Pending == nil {
		id, err := signer.NewID()
		if err != nil {
			return result, err
		}
		station := s.cfg.Station
		selected := info
		candidate := s.state
		candidate.Pending = &SigningSnapshot{RequestID: id, BroadcastID: info.BroadcastID, Identity: &selected, Station: &station, Online: s.cfg.Statistics.Online, Peak: s.cfg.Statistics.Peak}
		candidate.PendingSince = uint64(now.Unix())
		if e = s.store.Write(candidate); e != nil {
			return result, e
		}
		s.state = candidate
	}
	// Current authorization has been rechecked above. The original CERT must
	// remain pinned even if the public identity has rotated in the meantime.
	info, b, e = s.pendingIdentity()
	if e != nil {
		return result, e
	}
	if e = s.checkPendingCRLs(ctx, b); e != nil {
		return result, e
	}
	request := signer.SignRequest{RequestID: s.state.Pending.RequestID, Station: *s.state.Pending.Station, Online: s.state.Pending.Online, Peak: s.state.Pending.Peak}
	start := time.Now()
	response, e := s.machineSignature(ctx, info.BroadcastID, request)
	rtt := time.Since(start)
	if e != nil {
		if api, ok := e.(*signer.APIError); ok {
			switch api.Code {
			case "signature_expired", "signature_window_ending":
				if err := s.clearPending(); err != nil {
					return result, err
				}
				s.recovered = false
				return result, &RetryError{max(time.Second, api.RetryAfter), api.Code}
			}
		}
		return result, e
	}
	packet, e := signer.VerifySignature(info, b, request, response, s.now(), rtt)
	if e != nil {
		return result, e
	}
	packet = packet.LimitDeadline(s.crlDeadline)
	if !s.now().Before(packet.Deadline()) {
		return result, errors.New("verified CRL expired while preparing announcement")
	}
	if e = s.clearPending(); e != nil {
		return result, e
	}
	if s.recovered {
		s.recovered = false
		return result, &RetryError{30 * time.Second, "recovered previous signing request; discarded old announcement"}
	}
	result = CycleResult{Verified: true, Fingerprint: b.User.Fingerprint()}
	if !send {
		return result, nil
	}
	if ready != nil && b.User.Fingerprint() != s.state.Fingerprint {
		return result, &RetryError{time.Second, "SAS identity rotated while signing"}
	}
	if ready != nil && !ready() {
		return result, &RetryError{3 * time.Second, "mqtt_authentication_unavailable"}
	}
	s.state.LastAttempt = uint64(s.now().Unix())
	if e = s.store.Write(s.state); e != nil {
		return result, e
	}
	sender := aprs.Sender{Host: s.cfg.APRS.Host, Port: s.cfg.APRS.Port, PasscodeFile: s.cfg.APRS.PasscodeFile}
	if e = sender.Send(ctx, packet); e != nil {
		return result, &RetryError{600 * time.Second, "APRS delivery failed or unknown; no packet queued"}
	}
	s.state.LastPublished = uint64(s.now().Unix())
	result.Published = true
	return result, s.store.Write(s.state)
}
func (s *Service) Run(ctx context.Context, report func(CycleResult)) error {
	delay := time.Duration(0)
	failures := 0
	for {
		if e := Wait(ctx, delay); e != nil {
			return e
		}
		var result CycleResult
		var e error
		if s.state.EnrollmentID == "" {
			_, e = s.Enroll(ctx)
		}
		if e == nil {
			result, e = s.Cycle(ctx, true)
		}
		if e == nil {
			report(result)
			failures = 0
			delay = 600 * time.Second
			continue
		}
		var retry *RetryError
		if errors.As(e, &retry) {
			delay = retry.After
			continue
		}
		var temporary interface{ Temporary() bool }
		if errors.As(e, &temporary) && temporary.Temporary() {
			failures++
			delay = Backoff(failures)
			continue
		}
		var api *signer.APIError
		if errors.As(e, &api) && api.Retryable() {
			failures++
			delay = max(api.RetryAfter, Backoff(failures))
			continue
		}
		return e
	}
}
func Wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	base := time.Second * time.Duration(1<<uint(attempt-1))
	return base + time.Duration(time.Now().UnixNano()%int64(base/4+1))
}
