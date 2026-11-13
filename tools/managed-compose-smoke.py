"""Exercise the managed Compose stack using disposable Trust/APRS fixtures."""
import json
import os
import pathlib
import subprocess
import tempfile
import time

ROOT = pathlib.Path(__file__).resolve().parent.parent
ENV = dict(os.environ, BEACON_IMAGE="beacon-managed-integration", BEACON_CALLSIGN="AB1CD",
           BEACON_GRID="OL15ic", BEACON_NAME="Synthetic", BEACON_MQTT_HOST="voice.example.com",
           BEACON_OWNER="Test", BEACON_EMAIL="owner@example.com", BEACON_BIND_ADDR="127.0.0.1",
           BEACON_MQTT_PORT="0", BEACON_DASHBOARD_PORT="0")


def until(check, timeout=60):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        value = check()
        if value:
            return value
        time.sleep(0.5)
    raise AssertionError(f"Timed out waiting for {check.__name__}")


with tempfile.TemporaryDirectory(prefix="rf-link-compose-test-") as directory:
    override = pathlib.Path(directory) / "override.json"
    override.write_text(json.dumps({
        "services": {"beacon": {
            "entrypoint": ["/compose-fixture", "-test.run=^TestComposeFixture$", "-test.v"],
            "command": [],
            "environment": {"BEACON_COMPOSE_TEST": "1"},
        }},
        "networks": {"default": {"internal": True}},
    }))
    base = ["docker", "compose", "-p", f"rf-link-smoke-{os.getpid()}",
            "-f", str(ROOT / "compose.managed.yaml"), "-f", str(override)]

    def compose(*args):
        return subprocess.check_output(base + list(args), cwd=ROOT, env=ENV, text=True)

    def progress():
        return json.loads(compose("exec", "-T", "beacon", "/rf-link-server-beacon", "status"))

    def probe(expected):
        return subprocess.run(base + ["exec", "-T", "-e", f"BEACON_PROBE_EXPECT={expected}",
                                     "beacon", "/compose-fixture", "-test.run=^TestComposeMQTTProbe$"],
                              cwd=ROOT, env=ENV, capture_output=True)

    def must_probe(expected):
        result = probe(expected)
        assert result.returncode == 0, result.stdout.decode()

    def login():
        return probe("allow").returncode == 0

    try:
        compose("up", "-d", "--no-build")
        pending = until(lambda: (p if (p := progress())["phase"] == "pending_review" else None))
        assert pending["enrollmentId"]
        must_probe("closed")
        initial_password = compose("exec", "-T", "beacon", "/rf-link-server-beacon", "dashboard-password")
        assert len(initial_password.strip()) == 48
        beacon = compose("ps", "-q", "beacon").strip()
        approved = pathlib.Path(directory) / "approved"
        approved.write_text("1")
        subprocess.run(["docker", "cp", str(approved), beacon + ":/data/test-approved"], check=True)
        until(lambda: progress().get("lastPublished"), timeout=95)
        assert progress()["enrollmentId"] == pending["enrollmentId"]
        until(login)
        must_probe("deny")
        packet = pathlib.Path(directory) / "announcement"
        subprocess.run(["docker", "cp", beacon + ":/data/test-aprs", str(packet)], check=True)
        assert b"IFMO-V4,STATION,CERT:" in packet.read_bytes()
        compose("stop", "emqx")
        must_probe("closed")
        compose("start", "emqx")
        for _ in range(10):
            must_probe("blocked")
            time.sleep(0.3)
        until(login)
        assert compose("exec", "-T", "beacon", "/rf-link-server-beacon", "dashboard-password") == initial_password
        print("PASS: pending review, identity delivery, real SAS login, local APRS publication and broker restart")
    except Exception:
        print(compose("logs", "--tail", "80"))
        raise
    finally:
        compose("down", "-v")
