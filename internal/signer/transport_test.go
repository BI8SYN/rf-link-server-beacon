package signer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTransportFailureIsRetryableAndRedacted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _, e := w.(http.Hijacker).Hijack()
		if e == nil {
			c.Close()
		}
	}))
	defer server.Close()
	c, _ := NewApplication(server.URL, applicationToken, true)
	_, e := c.Poll(context.Background(), testID)
	var temporary interface{ Temporary() bool }
	if !errors.As(e, &temporary) || !temporary.Temporary() {
		t.Fatalf("network failure cannot be retried: %v", e)
	}
}

func TestInterruptedJSONBodyCanRetryOriginalRequest(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "1000")
		w.Write([]byte(`{"serverId":`))
	}))
	defer s.Close()
	c, _ := NewApplication(s.URL, applicationToken, true)
	_, e := c.Poll(context.Background(), testID)
	var temporary interface{ Temporary() bool }
	if !errors.As(e, &temporary) {
		t.Fatalf("interrupted response is not retryable: %v", e)
	}
}
