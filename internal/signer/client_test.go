package signer

import (
	"context"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testToken = "fmb1.550e8400-e29b-41d4-a716-446655440000.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
const testID = "95b976da-d670-44ec-bb72-b876ad05f134"

func TestHTTPStatusRetryAndSecrets(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		want   string
	}{{401, `{"error":{"code":"credential_invalid","message":"secret"},"correlationId":"id"}`, "credential_invalid"}, {403, `{"error":{"code":"signing_disabled","message":"secret"},"correlationId":"id"}`, "signing_disabled"}, {409, `{"error":{"code":"signature_expired","message":"secret"},"correlationId":"id"}`, "signature_expired"}, {429, `{"error":{"code":"rate_limited","message":"secret"},"correlationId":"id"}`, "rate_limited"}} {
		t.Run(tc.want, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/server-broadcast/enrollments/"+testID || r.Header.Get("Authorization") != "Bearer "+applicationToken {
					t.Error("wrong contract request")
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "37")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			c, e := NewApplication(server.URL, applicationToken, true)
			if e != nil {
				t.Fatal(e)
			}
			_, e = c.Poll(context.Background(), testID)
			api, ok := e.(*APIError)
			if !ok || api.Code != tc.want || api.RetryAfter != 37*time.Second || strings.Contains(e.Error(), "secret") || strings.Contains(e.Error(), testToken) {
				t.Fatalf("wrong safe error: %v", e)
			}
		})
	}
}
func TestRejectRedirectAndInvalidResponse(t *testing.T) {
	for _, body := range []string{`{}`, `{"serverId":"x","serverId":"y"}`, strings.Repeat("x", 65537)} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, body)
		}))
		c, _ := NewApplication(s.URL, applicationToken, true)
		if _, e := c.Poll(context.Background(), testID); e == nil {
			t.Fatal("malformed response accepted")
		}
		s.Close()
	}
	leaked := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer target.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer s.Close()
	c, _ := NewApplication(s.URL, applicationToken, true)
	if _, e := c.Poll(context.Background(), testID); e == nil || leaked {
		t.Fatal("redirect followed")
	}
}
func TestPOSTRetriesUseIdenticalBody(t *testing.T) {
	var first string
	n := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		r.Body.Read(b)
		if n == 0 {
			first = string(b)
		} else if string(b) != first {
			t.Error("idempotent request body changed")
		}
		n++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":{"code":"signing_unavailable","message":"unavailable"},"correlationId":"id"}`)
	}))
	defer s.Close()
	c, _ := New(s.URL, testToken, true)
	req := SignRequest{RequestID: testID, Station: protocol.Station{SSID: 12, Latitude: "2505.12N", Longitude: "10242.22E", SymbolTable: "/", SymbolCode: "I", CountryCode: "CN", Name: "Local", Host: "voice.example.com", Port: 1883, FilterKM: 5000}, Online: 2, Peak: 6}
	for i := 0; i < 2; i++ {
		c.SignStation(context.Background(), testID, req)
	}
	if n != 2 {
		t.Fatal("requests not attempted")
	}
}
