// Package signer implements the draft server-broadcast v1 HTTP contract.
package signer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"github.com/BI8SYN/rf-link-server-beacon/internal/identity"
	"github.com/BI8SYN/rf-link-server-beacon/internal/jsonx"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const prefix = "/api/v1/server-broadcast"

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func ValidID(s string) bool { return uuid.MatchString(s) }
func NewID() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

type Enrollment struct {
	Applicant   *config.Applicant `json:"applicant,omitempty"`
	RequestID   string            `json:"requestId"`
	Callsign    string            `json:"callsign"`
	Label       string            `json:"label"`
	PurposeNote string            `json:"purposeNote"`
	// Station is only retained for replaying an immutable pre-cutover application.
	Station *protocol.Station `json:"station,omitempty"`
}
type Created struct {
	ServerID     string `json:"serverId"`
	EnrollmentID string `json:"enrollmentId"`
	BroadcastID  string `json:"broadcastId"`
	State        string `json:"state"`
}
type Status struct {
	ServerID     string  `json:"serverId"`
	EnrollmentID string  `json:"enrollmentId"`
	BroadcastID  string  `json:"broadcastId"`
	State        string  `json:"state"`
	ReasonCode   string  `json:"reasonCode"`
	Reason       string  `json:"reason"`
	Fingerprint  *string `json:"certificateFingerprint"`
	PollAfter    uint64  `json:"pollAfterSeconds"`
}
type Identity struct {
	ServerID       string          `json:"serverId"`
	BroadcastID    string          `json:"broadcastId"`
	EnrollmentID   string          `json:"enrollmentId"`
	Callsign       string          `json:"callsign"`
	UID            uint64          `json:"uid"`
	Fingerprint    string          `json:"certificateFingerprint"`
	BlobHash       string          `json:"certBlobHash"`
	Blob           string          `json:"certBlob"`
	Bundle         identity.Bundle `json:"identityBundle"`
	SigningEnabled bool            `json:"signingEnabled"`
	ServerTime     uint64          `json:"serverTime"`
}
type SignRequest struct {
	RequestID string           `json:"requestId"`
	Station   protocol.Station `json:"station"`
	Online    uint64           `json:"online"`
	Peak      uint64           `json:"peak"`
}
type Signature struct {
	RequestID   string `json:"requestId"`
	BroadcastID string `json:"broadcastId"`
	Fingerprint string `json:"certificateFingerprint"`
	BlobHash    string `json:"certBlobHash"`
	TimeSalt    uint64 `json:"timeSalt"`
	SignedAt    uint64 `json:"signedAt"`
	ServerTime  uint64 `json:"serverTime"`
	SendBefore  uint64 `json:"sendBefore"`
	VerifyUntil uint64 `json:"verifyUntil"`
	Signature   string `json:"signature"`
	APRSLine    string `json:"aprsLine,omitempty"` // Legacy diagnostic only; never sent.
}
type APIError struct {
	Status               int
	Code                 string
	RetryAfter           time.Duration
	ExistingEnrollmentID string
	ExistingBroadcastID  string
}

func (e *APIError) Error() string   { return fmt.Sprintf("signer HTTP %d: %s", e.Status, e.Code) }
func (e *APIError) Retryable() bool { return e.Status == 429 || e.Status >= 500 }

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	CorrelationID string  `json:"correlationId"`
	Retry         *uint64 `json:"retryAfterSeconds,omitempty"`
	EnrollmentID  *string `json:"existingEnrollmentId,omitempty"`
	BroadcastID   *string `json:"existingBroadcastId,omitempty"`
}

// TransportError contains no URL, token, server body or underlying transport text.
type TransportError struct{}

func (*TransportError) Error() string   { return "remote service transport failed" }
func (*TransportError) Temporary() bool { return true }

type Client struct {
	base, token string
	kind, id    string
	http        *http.Client
	loopback    bool
}

func New(base, token string, loopback bool) (*Client, error) {
	return newClient(base, token, loopback, "fmb1")
}
func NewApplication(base, token string, loopback bool) (*Client, error) {
	return newClient(base, token, loopback, "fmba1")
}
func newClient(base, token string, loopback bool, kind string) (*Client, error) {
	if e := config.URL(base, loopback); e != nil {
		return nil, e
	}
	if strings.HasSuffix(base, "/") {
		return nil, errors.New("signer origin cannot end with slash")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != kind || !ValidID(parts[1]) {
		return nil, errors.New("invalid credential type or format")
	}
	if _, e := protocol.Decode64(parts[2], 32); e != nil {
		return nil, errors.New("invalid credential type or format")
	}
	return &Client{base: base, token: token, kind: kind, id: parts[1], http: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}, loopback: loopback}, nil
}
func NewApplicationToken(id string) (string, error) {
	if !ValidID(id) {
		return "", errors.New("invalid application ID")
	}
	secret := make([]byte, 32)
	if _, e := rand.Read(secret); e != nil {
		return "", e
	}
	return "fmba1." + id + "." + protocol.B64(secret), nil
}

type Credential struct {
	RequestID    string `json:"requestId"`
	EnrollmentID string `json:"enrollmentId"`
	BroadcastID  string `json:"broadcastId"`
	ServerID     string `json:"serverId"`
	CredentialID string `json:"credentialId"`
	MachineToken string `json:"machineToken"`
	ExpiresAt    uint64 `json:"expiresAt"`
	RetryUntil   uint64 `json:"retryUntil"`
}

func (c *Client) Claim(ctx context.Context, id, requestID string) (Credential, error) {
	var out Credential
	if c.kind != "fmba1" || id != c.id || !ValidID(requestID) || requestID == id {
		return out, errors.New("application receipt and separate claim request ID required")
	}
	request := struct {
		RequestID string `json:"requestId"`
	}{requestID}
	e := c.call(ctx, "POST", "/enrollments/"+id+"/credentials", request, &out, 200)
	if e != nil {
		return out, e
	}
	machine, err := New(c.base, out.MachineToken, c.loopback)
	if err != nil || out.RequestID != requestID || out.EnrollmentID != id || !ValidID(out.ServerID) || !ValidID(out.BroadcastID) || !ValidID(out.CredentialID) || machine.id != out.CredentialID || out.ExpiresAt == 0 || out.RetryUntil == 0 || out.RetryUntil > out.ExpiresAt {
		return Credential{}, errors.New("credential delivery binding failed")
	}
	return out, nil
}
func (c *Client) call(ctx context.Context, method, path string, body, out any, success ...int) error {
	var data []byte
	var e error
	if body != nil {
		data, e = json.Marshal(body)
		if e != nil || len(data) > 4096 {
			return errors.New("invalid signer request")
		}
	}
	req, e := http.NewRequestWithContext(ctx, method, c.base+prefix+path, bytes.NewReader(data))
	if e != nil {
		return errors.New("cannot create signer request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, e := c.http.Do(req)
	if e != nil {
		return &TransportError{}
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if e != nil {
		return &TransportError{}
	}
	if len(raw) > 65536 {
		return errors.New("signer response exceeds limit")
	}
	ct, _, e := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if e != nil || ct != "application/json" {
		return errors.New("signer returned non-JSON response")
	}
	for _, status := range success {
		if resp.StatusCode == status {
			if e = jsonx.Decode(raw, out); e != nil {
				return errors.New("invalid signer response fields")
			}
			return nil
		}
	}
	api := &APIError{Status: resp.StatusCode, Code: "unexpected_error"}
	var b errorBody
	if jsonx.Decode(raw, &b) == nil {
		if knownError(b.Error.Code) {
			api.Code = b.Error.Code
		}
		if b.Retry != nil && *b.Retry <= 86400 {
			api.RetryAfter = time.Duration(*b.Retry) * time.Second
		}
		if b.EnrollmentID != nil && ValidID(*b.EnrollmentID) {
			api.ExistingEnrollmentID = *b.EnrollmentID
		}
		if b.BroadcastID != nil && ValidID(*b.BroadcastID) {
			api.ExistingBroadcastID = *b.BroadcastID
		}
	}
	if n, e := strconv.ParseUint(resp.Header.Get("Retry-After"), 10, 32); e == nil && n <= 86400 {
		api.RetryAfter = time.Duration(n) * time.Second
	}
	return api
}
func knownError(s string) bool {
	switch s {
	case "application_access_invalid", "credential_recovery_pending", "credential_delivery_expired", "application_queue_full", "invalid_json", "invalid_request", "payload_too_large", "credential_invalid", "registration_disabled", "identity_unavailable", "signing_disabled", "registration_mismatch", "not_found", "broadcast_identity_exists", "request_conflict", "identity_not_ready", "signature_expired", "signature_window_ending", "packet_too_long", "rate_limited", "signing_unavailable":
		return true
	}
	return false
}
func (c *Client) Enroll(ctx context.Context, request Enrollment) (Created, error) {
	var out Created
	if c.kind != "fmba1" || request.RequestID != c.id || request.Applicant == nil || request.Applicant.Validate() != nil || !ValidID(request.RequestID) || !protocol.ValidCallsign(request.Callsign) || (request.Station != nil && request.Station.Validate() != nil) {
		return out, errors.New("invalid enrollment")
	}
	e := c.call(ctx, "POST", "/enrollments", request, &out, 200, 201)
	if e == nil && (!ValidID(out.ServerID) || out.EnrollmentID != request.RequestID || !ValidID(out.BroadcastID) || !validState(out.State)) {
		e = errors.New("enrollment response binding mismatch")
	}
	return out, e
}
func (c *Client) Poll(ctx context.Context, id string) (Status, error) {
	var out Status
	if c.kind != "fmba1" || id != c.id || !ValidID(id) {
		return out, errors.New("application receipt required for this enrollment")
	}
	e := c.call(ctx, "GET", "/enrollments/"+id, nil, &out, 200)
	if e == nil && (!ValidID(out.ServerID) || out.EnrollmentID != id || !ValidID(out.BroadcastID) || !validState(out.State) || out.PollAfter > 86400) {
		e = errors.New("invalid enrollment status")
	}
	return out, e
}
func validState(s string) bool {
	switch s {
	case "pending_review", "awaiting_signature", "publishing", "ready", "suspended", "rejected", "unavailable":
		return true
	}
	return false
}
func (c *Client) Identity(ctx context.Context, id string) (Identity, error) {
	var out Identity
	if c.kind != "fmb1" || !ValidID(id) {
		return out, errors.New("machine credential required for identity")
	}
	e := c.call(ctx, "GET", "/identities/"+id, nil, &out, 200)
	if e == nil && (!ValidID(out.ServerID) || !ValidID(out.EnrollmentID) || out.BroadcastID != id) {
		e = errors.New("identity response binding mismatch")
	}
	return out, e
}
func (c *Client) SignStation(ctx context.Context, id string, request SignRequest) (Signature, error) {
	var out Signature
	if c.kind != "fmb1" || !ValidID(id) || !ValidID(request.RequestID) || request.Station.Validate() != nil || request.Online > request.Peak || request.Peak > protocol.MaxInteger {
		return out, errors.New("invalid signature request")
	}
	e := c.call(ctx, "POST", "/identities/"+id+"/station-signatures", request, &out, 200)
	if e == nil && (out.RequestID != request.RequestID || out.BroadcastID != id) {
		e = errors.New("signature response binding mismatch")
	}
	return out, e
}

// PublicJSON never attaches a machine credential. Caller must enforce its URL allowlist.
func (c *Client) PublicJSON(ctx context.Context, source string, out any) error {
	if e := config.URL(source, c.loopback); e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "GET", source, nil)
	if e != nil {
		return errors.New("invalid public source")
	}
	resp, e := c.http.Do(req)
	if e != nil {
		return &TransportError{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("public PKI source unavailable")
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if e != nil || len(raw) > 1<<20 {
		return errors.New("public PKI response exceeds limit")
	}
	if e = jsonx.Decode(raw, out); e != nil {
		return errors.New("invalid public PKI JSON")
	}
	return nil
}
