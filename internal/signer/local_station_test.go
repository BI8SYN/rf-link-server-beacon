package signer

import (
	"context"
	"encoding/json"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"github.com/BI8SYN/rf-link-server-beacon/internal/jsonx"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewEnrollmentContainsOnlyIdentityReviewFields(t *testing.T) {
	req := Enrollment{RequestID: testID, Callsign: "BI8SYN", Label: "Identity", PurposeNote: "STATION", Applicant: &config.Applicant{Name: "Owner", Email: "owner@example.com", AuthorizationNote: "Test identity"}}
	raw, _ := json.Marshal(req)
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	if len(fields) != 5 || fields["station"] != nil {
		t.Fatal("new application still includes station")
	}
}

func TestSigningRejectsIncompleteStationAndLegacyProfileRequests(t *testing.T) {
	for _, raw := range []string{
		`{"requestId":"95b976da-d670-44ec-bb72-b876ad05f134","expectedProfileVersion":1,"online":0,"peak":0}`,
		`{"requestId":"95b976da-d670-44ec-bb72-b876ad05f134","station":{},"online":0,"peak":0}`,
		`{"requestId":"95b976da-d670-44ec-bb72-b876ad05f134","station":null,"online":0,"peak":0}`,
	} {
		var req SignRequest
		if jsonx.Decode([]byte(raw), &req) == nil {
			t.Fatal("legacy or incomplete request accepted")
		}
	}
}

func TestIdentityAndStatusRejectLegacyConfigurationResponses(t *testing.T) {
	for _, path := range []string{"identity", "status"} {
		t.Run(path, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				var out any
				if path == "identity" {
					raw, _ := json.Marshal(Identity{})
					var fields map[string]any
					json.Unmarshal(raw, &fields)
					fields["station"] = map[string]any{}
					fields["profileVersion"] = 1
					out = fields
				} else {
					out = map[string]any{"serverId": testID, "enrollmentId": testID, "broadcastId": testID, "state": "ready", "reasonCode": "", "reason": "", "certificateFingerprint": nil, "profileVersion": 1, "pollAfterSeconds": 30}
				}
				json.NewEncoder(w).Encode(out)
			}))
			defer server.Close()
			if path == "identity" {
				c, _ := New(server.URL, testToken, true)
				if _, e := c.Identity(context.Background(), testID); e == nil {
					t.Fatal("legacy identity response accepted")
				}
			} else {
				c, _ := NewApplication(server.URL, applicationToken, true)
				if _, e := c.Poll(context.Background(), testID); e == nil {
					t.Fatal("legacy status response accepted")
				}
			}
		})
	}
}

func TestLocalStationSigningRequestShape(t *testing.T) {
	raw := []byte(`{"requestId":"95b976da-d670-44ec-bb72-b876ad05f134","station":{"ssid":12,"latitude":"2505.12N","longitude":"10242.22E","symbolTable":"/","symbolCode":"I","countryCode":"CN","name":"Local station","host":"voice.example.com","port":1883,"filterKm":5000},"online":2,"peak":6}`)
	var req SignRequest
	if e := jsonx.Decode(raw, &req); e != nil {
		t.Fatal("new complete station request rejected", e)
	}
	out, _ := json.Marshal(req)
	var fields map[string]json.RawMessage
	json.Unmarshal(out, &fields)
	if len(fields) != 4 || fields["station"] == nil || fields["expectedProfileVersion"] != nil {
		t.Fatal("wrong signing wire shape")
	}
}
