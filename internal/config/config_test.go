package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func configMap(t *testing.T) map[string]any {
	t.Helper()
	raw, e := os.ReadFile("../../configs/beacon.advanced.example.json")
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]any
	json.Unmarshal(raw, &m)
	m["coordinates_confirmed"] = true
	m["trust"].(map[string]any)["root_fingerprints"] = []string{"ezhy0C8At6k0PZqi5hMPB-QVhfBpnfa3XltcrMcf77g"}
	m["callsign"] = "BI8SYN"
	m["signer"].(map[string]any)["base_url"] = "https://signer.bi8syn.com"
	m["station"].(map[string]any)["host"] = "voice.bi8syn.com"
	return m
}
func TestRejectInvalidAndInjectedConfig(t *testing.T) {
	for _, field := range []string{"unknown", "newline", "ssid", "port", "coordinates", "http", "placeholder"} {
		t.Run(field, func(t *testing.T) {
			m := configMap(t)
			s := m["station"].(map[string]any)
			switch field {
			case "unknown":
				m["extra"] = true
			case "newline":
				s["name"] = "hello\r\nworld"
			case "ssid":
				s["ssid"] = 16
			case "port":
				s["port"] = 0
			case "coordinates":
				m["coordinates_confirmed"] = false
			case "http":
				m["signer"].(map[string]any)["base_url"] = "http://signer.bi8syn.com"
			case "placeholder":
				m["callsign"] = "YOURCALL"
			}
			raw, _ := json.Marshal(m)
			p := filepath.Join(t.TempDir(), "config.json")
			os.WriteFile(p, raw, 0600)
			if _, e := Load(p); e == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}

func TestValidConfigurationAndPrivateSecrets(t *testing.T) {
	m := configMap(t)
	raw, _ := json.Marshal(m)
	p := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(p, raw, 0600)
	if _, e := Load(p); e != nil {
		t.Fatal("valid configuration rejected", e)
	}
	secret := filepath.Join(t.TempDir(), "token")
	os.WriteFile(secret, []byte("not-a-real-secret\n"), 0644)
	if _, e := Secret(secret); e == nil {
		t.Fatal("world-readable credential accepted")
	}
	os.Chmod(secret, 0600)
	if got, e := Secret(secret); e != nil || got != "not-a-real-secret" {
		t.Fatal(e)
	}
	link := secret + ".link"
	os.Symlink(secret, link)
	if _, e := Secret(link); e == nil {
		t.Fatal("symlink credential accepted")
	}
}
