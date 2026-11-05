package protocol

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The receiving app reads FMO-V4 at the byte immediately after the APRS
// position symbol. This expectation is independent of our line reconstruction.
func TestStationBodyStartsImmediatelyAfterPositionSymbol(t *testing.T) {
	s := Station{SSID: 12, Latitude: "2506.25N", Longitude: "10242.50E", SymbolTable: "/", SymbolCode: "I", CountryCode: "CN", Name: "RF Link", Host: "fmo.bi8syn.com", Port: 1883, FilterKM: 5000}
	c := UserCert{Callsign: "BI8SYN", PublicKey: make([]byte, 32), Signature: make([]byte, 64)}
	line, e := Line(c, s, 0, 0, B64(make([]byte, 64)))
	if e != nil {
		t.Fatal(e)
	}
	_, payload, ok := strings.Cut(line, ":")
	if !ok || !strings.HasPrefix(payload, "=2506.25N/10242.50EIFMO-V4,STATION,CERT:") {
		t.Fatal("strict app protocol parser cannot read body immediately after position symbol")
	}
}

// Removing a bound field or confusing the two hashes must fail this public vector.
func TestHistoricalStationVector(t *testing.T) {
	raw, _ := os.ReadFile("testdata/station-v1.json")
	var v struct {
		CertBlob    string `json:"certBlobBase64url"`
		Fingerprint string `json:"certificateFingerprint"`
		Hash        string `json:"certBlobHash"`
		TBS         string `json:"tbsCborHex"`
		Signature   string `json:"signatureBase64url"`
		Public      string `json:"publicKeyBase64url"`
		Salt        uint64 `json:"timeSalt"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	cert, err := ParseBlob(v.CertBlob)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Fingerprint() != v.Fingerprint || cert.BlobHash() != v.Hash || v.Fingerprint == v.Hash {
		t.Fatal("certificate hash binding")
	}
	s := Station{SSID: 15, Latitude: "2505.12N", Longitude: "10242.22E", SymbolTable: "/", SymbolCode: "I", CountryCode: "CN", Name: "RF Link", Host: "fmo.bi8syn.com", Port: 1883, FilterKM: 5000}
	got := StationTBS(cert, s, 2, 6, v.Salt)
	want, _ := hex.DecodeString(v.TBS)
	if !bytes.Equal(got, want) {
		t.Fatalf("TBS mismatch: %x", got)
	}
	sig, _ := Decode64(v.Signature, 64)
	pub, _ := Decode64(v.Public, 32)
	if !ed25519.Verify(pub, got, sig) {
		t.Fatal("historical signature rejected")
	}
	for _, field := range []string{"host", "port", "name", "ssid", "latitude", "longitude", "online", "peak", "salt"} {
		t.Run(field, func(t *testing.T) {
			mutated := s
			o, p, salt := uint64(2), uint64(6), v.Salt
			switch field {
			case "host":
				mutated.Host = "other.bi8syn.com"
			case "port":
				mutated.Port++
			case "name":
				mutated.Name = "Other"
			case "ssid":
				mutated.SSID++
			case "latitude":
				mutated.Latitude = "2505.13N"
			case "longitude":
				mutated.Longitude = "10242.23E"
			case "online":
				o++
			case "peak":
				p++
			case "salt":
				salt += 2
			}
			if ed25519.Verify(pub, StationTBS(cert, mutated, o, p, salt), sig) {
				t.Fatal("tamper accepted")
			}
		})
	}
}
func TestRejectNoncanonicalBlob(t *testing.T) {
	for _, s := range []string{"gQ==", "_w", "nw", "gAA"} {
		if _, err := ParseBlob(s); err == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}
func TestStationValidation(t *testing.T) {
	s := Station{SSID: 15, Latitude: "2505.12N", Longitude: "10242.22E", SymbolTable: "/", SymbolCode: "I", CountryCode: "CN", Name: "测试", Host: "voice.example.com", Port: 1883, FilterKM: 5000}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"injection", "latitude", "longitude", "ssid", "port", "host"} {
		t.Run(field, func(t *testing.T) {
			bad := s
			switch field {
			case "injection":
				bad.Name = "foo\r\nbar"
			case "latitude":
				bad.Latitude = "9000.01N"
			case "longitude":
				bad.Longitude = "18000.01E"
			case "ssid":
				bad.SSID = 16
			case "port":
				bad.Port = 0
			case "host":
				bad.Host = "https://voice.example.com"
			}
			if bad.Validate() == nil {
				t.Fatal("invalid station accepted")
			}
		})
	}
}
