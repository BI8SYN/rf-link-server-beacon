package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCollectsContactsWithoutRequestingMachineToken(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	var out, errOut bytes.Buffer
	args := []string{"init", "--data-dir", dir, "--callsign", "BI8SYN", "--ssid", "0", "--grid", "FN31pr", "--name", "Test", "--host", "voice.bi8syn.com"}
	if n := RunWithInput(args, strings.NewReader("Test Owner\nowner@example.com\n"), &out, &errOut, "test"); n != 0 {
		t.Fatalf("init %d %s", n, errOut.String())
	}
	raw, e := os.ReadFile(filepath.Join(dir, "config.json"))
	if e != nil {
		t.Fatal(e)
	}
	var fields map[string]any
	json.Unmarshal(raw, &fields)
	if fields["ssid"] != float64(0) || fields["applicant"] == nil {
		t.Fatal("contact or explicit SSID0 missing")
	}
	if strings.Contains(out.String()+errOut.String(), "Machine credential") {
		t.Fatal("manual credential workflow still required")
	}
	if _, e = os.Stat(filepath.Join(dir, "credential")); !os.IsNotExist(e) {
		t.Fatal("init created machine credential")
	}
	if n := RunWithInput([]string{"activate", "--data-dir", dir}, strings.NewReader(""), &out, &errOut, "test"); n != 2 {
		t.Fatal("manual activation entry still exposed")
	}
	if n := RunWithInput(append(args, "--owner", "Test", "--email", "owner@example.com"), strings.NewReader(""), &out, &errOut, "test"); n == 0 {
		t.Fatal("existing configuration replaced")
	}
}
