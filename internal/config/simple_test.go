package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSimpleConfigExpandsDefaultsInsideOneDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"callsign":"bi8syn","ssid":15,"grid":"FN31pr","server":{"name":"测试节点","host":"VOICE.BI8SYN.COM","port":1883}}`), 0600)
	c, e := Load(path)
	if e != nil {
		t.Fatal(e)
	}
	if c.Callsign != "BI8SYN" || c.Station.Latitude != "4143.75N" || c.Station.Longitude != "07242.50W" || c.Station.Host != "voice.bi8syn.com" || c.APRS.PasscodeFile != "" || c.APRS.Interval != 600 || c.StateDir != filepath.Join(dir, "state") || c.Signer.CredentialFile != filepath.Join(dir, "credential") {
		t.Fatalf("wrong simplified expansion: %#v", c)
	}
	if len(c.Trust.Roots) != 1 || len(c.Trust.CRLOrigins) != 1 {
		t.Fatal("trusted provider defaults missing")
	}
	if !c.AllowCRL("https://pki.bi8syn.com/fmo/v1/ca-test/root.crl.json") || c.AllowCRL("https://attacker.invalid/root.crl.json") {
		t.Fatal("CRL discovery source policy")
	}
}
func TestRejectInvalidSimpleConfig(t *testing.T) {
	for _, raw := range []string{`{"callsign":"BI8SYN","ssid":16,"grid":"FN31pr","server":{"name":"Test","host":"voice.bi8syn.com"}}`, `{"callsign":"BI8SYN","ssid":15,"grid":"FN31zz","server":{"name":"Test","host":"voice.bi8syn.com"}}`, `{"callsign":"BI8SYN","ssid":15,"grid":"FN31pr","server":{"name":"Test","host":"voice.bi8syn.com","port":0}}`, `{"callsign":"BI8SYN","ssid":15,"grid":"FN31pr","server":{"name":"Test","host":"voice.bi8syn.com"},"token":"must-not-be-here"}`} {
		p := filepath.Join(t.TempDir(), "config.json")
		os.WriteFile(p, []byte(raw), 0600)
		if _, e := Load(p); e == nil {
			t.Fatalf("invalid simple config accepted: %s", raw)
		}
	}
}
func TestInitializeDoesNotOverwriteConfiguration(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	simple := Simple{Callsign: "BI8SYN", SSID: 15, Grid: "FN31pr", Server: Server{Name: "Test", Host: "voice.bi8syn.com", Port: 1883}}
	if e := Initialize(dir, simple); e != nil {
		t.Fatal(e)
	}
	simple.Callsign = "OTHER"
	if e := Initialize(dir, simple); e == nil {
		t.Fatal("existing configuration overwritten")
	}
	c, e := Load(filepath.Join(dir, "config.json"))
	if e != nil || c.Callsign != "BI8SYN" {
		t.Fatal(e)
	}
}

func TestInitializePreservesSSIDZeroAndFilterZero(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	s := Simple{Callsign: "BI8SYN", SSID: 0, Grid: "FN31pr", Server: Server{Name: "Test", Host: "voice.bi8syn.com", Port: 1883}, FilterKM: 0}
	if e := Initialize(dir, s); e != nil {
		t.Fatal(e)
	}
	c, e := Load(filepath.Join(dir, "config.json"))
	if e != nil || c.Station.SSID != 0 || c.Station.FilterKM != 0 {
		t.Fatalf("explicit zero changed by defaults: %d %d %v", c.Station.SSID, c.Station.FilterKM, e)
	}
}

func TestExplicitPublicDevRootIsCopiedIntoData(t *testing.T) {
	raw, e := os.ReadFile("../protocol/testdata/identity-bundle.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixture struct {
		Bundle struct {
			Root json.RawMessage `json:"rootCert"`
		} `json:"bundle"`
		Fingerprint string `json:"trustedRootFingerprint"`
	}
	if e = json.Unmarshal(raw, &fixture); e != nil {
		t.Fatal(e)
	}
	source := filepath.Join(t.TempDir(), "approved-root.json")
	os.WriteFile(source, fixture.Bundle.Root, 0644)
	dir := filepath.Join(t.TempDir(), "data")
	s := Simple{Callsign: "AB1CD", SSID: 15, Grid: "FN31pr", Server: Server{Name: "Test", Host: "voice.example.com", Port: 1883}, CountryCode: "US", FilterKM: 5000, ServiceURL: "http://localhost:18105", TrustedRootFile: source}
	if e = Initialize(dir, s); e != nil {
		t.Fatal(e)
	}
	c, e := Load(filepath.Join(dir, "config.json"))
	if e != nil || c.Trust.Roots[0] != fixture.Fingerprint || !c.Signer.AllowLoopbackHTTP {
		t.Fatal("explicit root import failed", e)
	}
	if _, e = os.Stat(filepath.Join(dir, "trusted-root.json")); e != nil {
		t.Fatal("custom public root left outside data", e)
	}
	if c.AllowCRL("http://192.168.1.1/secret") {
		t.Fatal("arbitrary private endpoint allowed")
	}
}
