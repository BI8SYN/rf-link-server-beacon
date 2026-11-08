#!/usr/bin/env python3
"""Load synthetic identities in the actual pinned SAS, with networking disabled."""
import importlib.util
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import time

ROOT = pathlib.Path(__file__).resolve().parent.parent
SAS = 'ghcr.io/bg5esn/fmo-server-authrozier-service:1.0.6@sha256:4145c2a65bff74125f1911d602f7988446b88dfd721ce139cd17a5952dd56323'
spec = importlib.util.spec_from_file_location('sas_config', ROOT/'tools/sas-config.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def docker(*args):
    return subprocess.check_output(['docker', *args], text=True, stderr=subprocess.STDOUT)


with tempfile.TemporaryDirectory(prefix='beacon-single-sas-') as tmp:
    tmp = pathlib.Path(tmp)
    original = json.loads((ROOT/'deploy/sas/config.example.json').read_text())
    original['Server'].update(Uid=10, Callsign='AB1CD', CertFingerprint='B'*43,
                              Admins=[dict(uid=11, certFingerprint='C'*42+'A', role='admin')])
    original['Mqtt'].update(Host='mqtt.example.invalid', Port=1883)
    original['Trust']['RootsDir'] = '/sasdata/roots'
    cluster = dict(uid=20, callsign='AB1CD', mqtt_host='mqtt.example.invalid',
                   mqtt_port=1883, certFingerprint='A'*43)
    candidate = module.merge(original, cluster)
    path = tmp/'config.json'
    path.write_text(json.dumps(candidate))
    path.chmod(0o644)
    # Config stays read-only; roots/roles use a disposable writable directory.
    # Only public roots and synthetic identities; networking is disabled.
    shutil.copytree(ROOT/'deploy/trust/roots', tmp/'sasdata'/'roots')
    name = 'beacon-single-sas-' + str(time.time_ns())
    # Match the bind-mount owner: Linux preserves tempfile's 0700 directory
    # permissions, while the image normally runs as app:1654. Keep that privacy
    # boundary instead of making temporary files readable by all host users.
    args = ['--user', str(os.getuid())+':'+str(os.getgid()),
            '--network', 'none', '--read-only', '--cap-drop', 'ALL',
            '--security-opt', 'no-new-privileges:true',
            '--mount', 'type=bind,src='+str(tmp)+',dst=/test,readonly',
            '--mount', 'type=bind,src='+str(tmp/'sasdata')+',dst=/sasdata']
    try:
        docker('run', '-d', '--name', name, *args, SAS, '--config', '/test/config.json')
        deadline = time.monotonic()+30
        while True:
            logs = docker('logs', name)
            if 'SAS HTTP auth listening on:' in logs:
                break
            if time.monotonic() > deadline or docker('inspect', '--format', '{{.State.Running}}', name).strip() != 'true':
                raise AssertionError('SAS startup failed: '+logs)
            time.sleep(0.3)
        assert 'Server: uid=10 callsign=AB1CD' in logs, logs
        assert 'uid=20 callsign=AB1CD mqtt=mqtt.example.invalid:1883' in logs, logs
        assert 'Clusters: 1 entries' in logs, logs
        assert 'Root CA store initialized with 2 root(s)' in logs, logs
        assert 'sn=8569869802686805' in logs and 'sn=1 keyId=1' in logs, logs
        time.sleep(0.5)
        assert docker('inspect', '--format', '{{.State.Running}}', name).strip() == 'true'
        assert json.loads(path.read_text()) == candidate, 'SAS modified read-only candidate'
        assert candidate['Server'] == original['Server']
        assert candidate['Trust'] == original['Trust']
        print('PASS: actual SAS loaded original primary and added Cluster on the same endpoint; no network or runtime config writes')
    finally:
        subprocess.run(['docker', 'rm', '-f', name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
