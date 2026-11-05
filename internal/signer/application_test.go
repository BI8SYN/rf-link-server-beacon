package signer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const applicationToken = "fmba1.95b976da-d670-44ec-bb72-b876ad05f134.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func TestApplicationCredentialsUseSeparateAuthorization(t *testing.T) {
	const claimID = "91717f1b-91f3-40c9-91f0-41dbf582bf48"
	requests := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer "+applicationToken {
			t.Error("wrong credential domain")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case prefix + "/enrollments/" + testID + "/credentials":
			var request map[string]string
			json.NewDecoder(r.Body).Decode(&request)
			if request["requestId"] != claimID || len(request) != 1 {
				t.Error("wrong claim payload")
			}
			fmt.Fprintf(w, `{"requestId":%q,"enrollmentId":%q,"broadcastId":%q,"serverId":%q,"credentialId":"550e8400-e29b-41d4-a716-446655440000","machineToken":%q,"expiresAt":1794000000,"retryUntil":1793319300}`, claimID, testID, testID, testID, testToken)
		default:
			t.Error("unexpected request")
		}
	}))
	defer s.Close()
	c, e := NewApplication(s.URL, applicationToken, true)
	if e != nil {
		t.Fatal(e)
	}
	claim, e := c.Claim(context.Background(), testID, claimID)
	if e != nil || claim.MachineToken != testToken {
		t.Fatal("credential delivery failed", e)
	}
	if _, e = c.Identity(context.Background(), testID); e == nil {
		t.Fatal("application token used as machine credential")
	}
	machine, _ := New(s.URL, testToken, true)
	if _, e = machine.Poll(context.Background(), testID); e == nil {
		t.Fatal("machine token used to query application")
	}
	if requests != 1 {
		t.Fatal("role mismatch made an HTTP request")
	}
}
func TestApplicationSecretGeneratedLocallyAndBindsID(t *testing.T) {
	a, e := NewApplicationToken(testID)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := NewApplicationToken(testID)
	if a == b || !strings.HasPrefix(a, "fmba1."+testID+".") {
		t.Fatal("application access is not independent random material")
	}
	if _, e = New("http://localhost:18105", a, true); e == nil {
		t.Fatal("receipt accepted as machine token")
	}
	if _, e = NewApplication("http://localhost:18105", testToken, true); e == nil {
		t.Fatal("machine token accepted as receipt")
	}
	app, _ := NewApplication("http://localhost:18105", a, true)
	if _, e = app.Poll(context.Background(), "550e8400-e29b-41d4-a716-446655440000"); e == nil {
		t.Fatal("receipt accepted for another request ID")
	}
}
func TestApplicantValidation(t *testing.T) {
	a := config.Applicant{Name: "Test Owner", Email: "owner@example.com", AuthorizationNote: "Synthetic test data; do not approve as real ownership"}
	if e := a.Validate(); e != nil {
		t.Fatal(e)
	}
	a.Email = "invalid"
	if a.Validate() == nil {
		t.Fatal("invalid applicant email accepted")
	}
}
