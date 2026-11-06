package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestNetworkCommandsRequireExplicitConfiguration(t *testing.T) {
	for _, cmd := range []string{"enroll", "run", "once", "doctor", "sync-identity"} {
		t.Run(cmd, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if Run([]string{cmd}, &out, &errOut, "test") == 0 || !strings.Contains(errOut.String(), "config") {
				t.Fatal("network operation accepted without configuration")
			}
		})
	}
}
func TestRejectUnexpectedArguments(t *testing.T) {
	var out, errOut bytes.Buffer
	if Run([]string{"run", "--bogus"}, &out, &errOut, "test") != 2 {
		t.Fatal("must reject unknown flags")
	}
}
func TestStatusDoesNotClaimProductionReady(t *testing.T) {
	var out, errOut bytes.Buffer
	if Run([]string{"status"}, &out, &errOut, "test") != 0 || !strings.Contains(out.String(), `"production_ready":false`) {
		t.Fatal("must report remaining integration boundary")
	}
}

func TestSASDeploymentValidatedBeforeConfigurationRead(t *testing.T) {
	for _, mode := range []string{"", "auto", "unknown"} {
		var out, errOut bytes.Buffer
		args := []string{"sas-command", "--config", "/nonexistent/config.json"}
		if mode != "" {
			args = append(args, "--deployment", mode)
		}
		if code := Run(args, &out, &errOut, "test"); code != 2 || !strings.Contains(errOut.String(), "deployment") {
			t.Fatalf("invalid deployment not rejected before config: %s (%d)", errOut.String(), code)
		}
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"doctor", "--config", "/nonexistent/config.json", "--deployment", "native"}, &out, &errOut, "test"); code != 2 {
		t.Fatal("deployment accepted on other command")
	}
}
