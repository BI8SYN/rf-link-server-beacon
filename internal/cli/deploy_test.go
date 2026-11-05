package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerDeployInitializesOnceThenReusesExistingIdentity(t *testing.T) {
	root, _ := filepath.Abs("../..")
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "bin")
	os.Mkdir(bin, 0700)
	log := filepath.Join(tmp, "docker.log")
	data := filepath.Join(tmp, "data")
	docker := `#!/bin/sh
printf '%s\n' "$*" >> "$TEST_DOCKER_LOG"
case " $* " in
 *" init "*) printf '%s\n' '{"callsign":"AB1CD","ssid":12}' > "$BEACON_DATA_DIR/config.json" ;;
esac
`
	if e := os.WriteFile(filepath.Join(bin, "docker"), []byte(docker), 0700); e != nil {
		t.Fatal(e)
	}
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command("sh", append([]string{filepath.Join(root, "beacon.sh"), "deploy-docker"}, args...)...)
		cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "BEACON_DATA_DIR="+data, "TEST_DOCKER_LOG="+log)
		out, e := cmd.CombinedOutput()
		return string(out), e
	}
	args := []string{"--callsign", "AB1CD", "--ssid", "12", "--grid", "FN31pr", "--name", "Test", "--host", "voice.example.com", "--owner", "Owner", "--email", "owner@example.com"}
	if out, e := run(args...); e != nil {
		t.Fatal("one-command Docker deployment failed", out, e)
	}
	before, _ := os.ReadFile(filepath.Join(data, "config.json"))
	lines, _ := os.ReadFile(log)
	if !strings.Contains(string(lines), "build beacon") || !strings.Contains(string(lines), "up -d beacon") || strings.Count(string(lines), " init ") != 1 {
		t.Fatal("build/init/background start incomplete", string(lines))
	}
	if out, e := run(); e != nil {
		t.Fatal("existing identity could not be deployed", out, e)
	}
	after, _ := os.ReadFile(filepath.Join(data, "config.json"))
	if string(after) != string(before) {
		t.Fatal("migration replaced configuration")
	}
	lines, _ = os.ReadFile(log)
	if strings.Count(string(lines), " init ") != 1 {
		t.Fatal("migration reinitialized application")
	}
	if _, e := run("--ssid", "13"); e == nil {
		t.Fatal("existing config silently accepted conflicting init flags")
	}
}
