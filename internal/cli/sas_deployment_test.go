package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSASCommandHostDetectionAndExplicitOverride(t *testing.T) {
	root, _ := filepath.Abs("../..")
	cases := []struct {
		name, containers string
		native           bool
		args             []string
		want             string
		fail             bool
	}{
		{name: "own-stack", containers: "ghcr.io/bg5esn/fmo-server-authrozier-service:1.0.6|sas|" + root + "|" + root + "/compose.yaml", want: "full-stack"},
		{name: "different-file-same-directory", containers: "ghcr.io/bg5esn/fmo-server-authrozier-service:1.0.6|sas|" + root + "|" + root + "/custom.yaml", want: "compose"},
		{name: "other-compose", containers: "ghcr.io/bg5esn/fmo-server-authrozier-service:1.0.6|sas|/srv/other", want: "compose"},
		{name: "standalone-container", containers: "ghcr.io/bg5esn/fmo-server-authrozier-service:1.0.6||", want: "docker"},
		{name: "native", native: true, want: "native"},
		{name: "beacon-docker-is-not-sas", containers: "rf-link-beacon:dev|beacon|" + root, fail: true},
		{name: "unknown", fail: true},
		{name: "ambiguous", native: true, containers: "ghcr.io/bg5esn/fmo-server-authrozier-service:1.0.6|sas|/srv/other", fail: true},
		{name: "explicit", args: []string{"--deployment", "full-stack"}, want: "full-stack"},
		{name: "explicit-equals", args: []string{"--deployment=native"}, want: "native"},
		{name: "invalid", args: []string{"--deployment", "bogus"}, fail: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "bin")
			os.Mkdir(bin, 0700)
			log := filepath.Join(dir, "docker.log")
			scripts := map[string]string{
				"docker":    "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$TEST_DOCKER_LOG\"\n[ \"$1\" = ps ] || exit 99\nprintf '%s\\n' \"$TEST_CONTAINERS\"\n",
				"systemctl": "#!/bin/sh\n[ \"$TEST_NATIVE_SAS\" = 1 ]\n",
				"beacon":    "#!/bin/sh\nprintf '%s\\n' \"$*\"\n",
			}
			for name, script := range scripts {
				if e := os.WriteFile(filepath.Join(bin, name), []byte(script), 0700); e != nil {
					t.Fatal(e)
				}
			}
			native := "0"
			if tc.native {
				native = "1"
			}
			args := append([]string{filepath.Join(root, "beacon.sh"), "sas-command"}, tc.args...)
			cmd := exec.Command("/bin/sh", args...)
			cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin", "BEACON_NATIVE=1", "BEACON_BINARY="+filepath.Join(bin, "beacon"), "BEACON_DATA_DIR="+filepath.Join(dir, "data"), "TEST_NATIVE_SAS="+native, "TEST_CONTAINERS="+tc.containers, "TEST_DOCKER_LOG="+log)
			out, e := cmd.CombinedOutput()
			if tc.fail {
				if e == nil {
					t.Fatalf("expected failure, got %s", out)
				}
				return
			}
			if e != nil || !strings.Contains(string(out), "--deployment "+tc.want) {
				t.Fatalf("wrong deployment: %s %v", out, e)
			}
			if len(tc.args) > 0 {
				if b, _ := os.ReadFile(log); len(b) != 0 {
					t.Fatal("explicit selection probed Docker")
				}
			}
		})
	}
}
