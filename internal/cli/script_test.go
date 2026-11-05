package cli

import (
	"bytes"
	"encoding/json"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSetupScriptInitializesAndEnrollsIdentityOnlyWithoutAPRS(t *testing.T) {
	const id = "95b976da-d670-44ec-bb72-b876ad05f134"
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != "POST" || r.URL.Path != "/api/v1/server-broadcast/enrollments" {
			t.Error("setup performed an unexpected request")
			http.NotFound(w, r)
			return
		}
		var request signer.Enrollment
		if e := json.NewDecoder(r.Body).Decode(&request); e != nil {
			t.Error(e)
		}
		if request.Callsign != "AB1CD" || request.Station != nil {
			t.Error("application leaked local station configuration")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(signer.Created{ServerID: id, EnrollmentID: request.RequestID, BroadcastID: id, State: "pending_review"})
	}))
	defer server.Close()
	root, _ := filepath.Abs("../..")
	dir := filepath.Join(t.TempDir(), "data")
	binary := filepath.Join(t.TempDir(), "beacon")
	cmd := exec.Command("sh", filepath.Join(root, "beacon.sh"), "setup", "--callsign", "AB1CD", "--ssid", "15", "--grid", "FN31pr", "--name", "Script Test", "--host", "voice.example.com", "--service-url", server.URL, "--owner", "Test Owner", "--email", "owner@example.com")
	cmd.Env = append(os.Environ(), "BEACON_NATIVE=1", "BEACON_DATA_DIR="+dir, "BEACON_BINARY="+binary)
	const token = "fmb1.550e8400-e29b-41d4-a716-446655440000.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	cmd.Stdin = strings.NewReader("")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if e := cmd.Run(); e != nil {
		t.Fatalf("setup script failed: %v\n%s", e, out.String())
	}
	if strings.Contains(out.String(), token) {
		t.Fatal("script printed machine credential")
	}
	if requests.Load() != 1 {
		t.Fatalf("setup did not stop after the application: %d requests", requests.Load())
	}
	if _, e := os.Stat(filepath.Join(dir, "config.json")); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(dir, "credential")); !os.IsNotExist(e) {
		t.Fatal("pending application created a manual machine token file")
	}
	if _, e := os.Stat(filepath.Join(dir, "state", "state.json")); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(dir, "aprs_passcode")); !os.IsNotExist(e) {
		t.Fatal("unnecessary APRS password file created")
	}
}
