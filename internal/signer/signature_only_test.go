package signer

import (
	"context"
	"encoding/json"
	"github.com/BI8SYN/rf-link-server-beacon/internal/jsonx"
	"github.com/BI8SYN/rf-link-server-beacon/internal/protocol"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignatureResponseAcceptsTenFieldsAndLegacyDiagnostic(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "signature_only", true: "legacy_text"}[legacy], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fields := signatureResponseFields(t)
				if legacy {
					fields["aprsLine"] = "untrusted legacy I FMO-V4 text"
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(fields)
			}))
			defer server.Close()
			c, e := New(server.URL, testToken, true)
			if e != nil {
				t.Fatal(e)
			}
			request := SignRequest{RequestID: testID, Station: protocol.Station{SSID: 12, Latitude: "2506.25N", Longitude: "10242.50E", SymbolTable: "/", SymbolCode: "I", CountryCode: "CN", Name: "Test", Host: "voice.example.com", Port: 1883, FilterKM: 5000}}
			if _, e = c.SignStation(context.Background(), testID, request); e != nil {
				t.Fatal("response transition rejected", e)
			}
		})
	}
}

func TestSignatureResponseStillRequiresAllTenMetadataFields(t *testing.T) {
	for missing := range signatureResponseFields(t) {
		t.Run(missing, func(t *testing.T) {
			fields := signatureResponseFields(t)
			delete(fields, missing)
			raw, _ := json.Marshal(fields)
			var result Signature
			if jsonx.Decode(raw, &result) == nil {
				t.Fatal("missing signed metadata accepted")
			}
		})
	}
	for _, bad := range []struct {
		key   string
		value any
	}{{"unknown", true}, {"aprsLine", 123}, {"aprsLine", nil}} {
		fields := signatureResponseFields(t)
		fields[bad.key] = bad.value
		raw, _ := json.Marshal(fields)
		var result Signature
		if jsonx.Decode(raw, &result) == nil {
			t.Fatal("unknown or malformed diagnostic field accepted")
		}
	}
}

func signatureResponseFields(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{
		"requestId": testID, "broadcastId": testID,
		"certificateFingerprint": protocol.Hash([]byte("cert-tbs")),
		"certBlobHash":           protocol.Hash([]byte("cert-blob")),
		"timeSalt":               uint64(2989787), "signedAt": uint64(1793872262),
		"serverTime": uint64(1793872262), "sendBefore": uint64(1793872322),
		"verifyUntil": uint64(1793873400), "signature": protocol.B64(make([]byte, 64)),
	}
}
