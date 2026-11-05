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
