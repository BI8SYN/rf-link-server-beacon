#!/usr/bin/env python3
"""Opt-in isolated EMQX 5.8.9 listener/auth/ACL regression. No FMO/Trust/APRS traffic."""
import json, os, pathlib, socket, struct, subprocess, tempfile, time

ROOT = pathlib.Path(__file__).resolve().parent.parent
EMQX = 'emqx/emqx:5.8.9@sha256:35b46f7aa7a0d51f3959e4344221ac5ec7f948b03e43099286b6c40a07bb393b'
SAS = 'ghcr.io/bg5esn/fmo-server-authrozier-service:1.0.6@sha256:4145c2a65bff74125f1911d602f7988446b88dfd721ce139cd17a5952dd56323'
PREFIX = 'beacon-coexist-test-' + str(os.getpid())
containers = []

def docker(*args):
    return subprocess.check_output(['docker', *args], text=True).strip()

def start(name, *args):
    full = PREFIX + '-' + name
    docker('run', '-d', '--name', full, '--network', PREFIX, *args)
    containers.append(full)
    return full

def port(name, internal):
    value = json.loads(docker('inspect', '--format', '{{json .NetworkSettings.Ports}}', name))
    return int(value[str(internal) + '/tcp'][0]['HostPort'])

def field(s):
    b = s.encode(); return struct.pack('!H', len(b)) + b

def packet(header, body):
    n = len(body); length = bytearray()
    while True:
        b = n % 128; n //= 128; length.append(b | (128 if n else 0))
        if not n: return bytes([header]) + length + body

def exact(s, n):
    out = b''
    while len(out) < n:
        b = s.recv(n - len(out))
        if not b: raise AssertionError('unexpected MQTT EOF')
        out += b
    return out

def receive(s):
    header = exact(s, 1)[0]; n = 0; mul = 1
    for _ in range(4):
        b = exact(s, 1)[0]; n += (b & 127) * mul
        if not b & 128: return header, exact(s, n)
        mul *= 128
    raise AssertionError('bad MQTT length')

def connect(p, identity, expected=0):
    s = socket.create_connection(('127.0.0.1', p), timeout=5)
    body = field('MQTT') + bytes([4, 194 if identity else 2]) + b'\x00\x1e' + field('smoke-' + identity + '-' + str(time.time_ns()))
    if identity: body += field(identity) + field('synthetic-test-only')
    s.sendall(packet(0x10, body)); header, body = receive(s)
    assert header == 0x20 and len(body) == 2, (header, body)
    if expected:
        assert body[1] in (4, 5), ('unexpected allow', identity, p, body)
        s.close(); return None
    assert body[1] == 0, ('unexpected deny', identity, p, body)
    return s

def exchange(old_port, new_port):
    for sub_port, sub_role, pub_port, pub_role in [(old_port,'fmo',new_port,'rflink'),(new_port,'rflink',old_port,'fmo')]:
        sub = connect(sub_port, sub_role); pub = connect(pub_port, pub_role)
        try:
            topic = 'room/smoke'
            sub.sendall(packet(0x82, b'\x00\x01' + field(topic) + b'\x00'))
            h, body = receive(sub); assert h == 0x90 and body[-1] == 0, (h,body)
            pub.sendall(packet(0x30, field(topic) + b'cross-port-ok'))
            h, body = receive(sub); assert h == 0x30 and body == field(topic) + b'cross-port-ok'
            sub.sendall(packet(0x82, b'\x00\x02' + field('forbidden/private') + b'\x00'))
            h, body = receive(sub); assert h == 0x90 and body[-1] == 128, ('ACL bypass', h, body)
        finally: sub.close(); pub.close()

with tempfile.TemporaryDirectory(prefix=PREFIX) as tmp:
    tmp = pathlib.Path(tmp)
    arch = docker('info', '--format', '{{.Architecture}}')
    env = dict(os.environ, CGO_ENABLED='0', GOOS='linux', GOARCH={'aarch64':'arm64','arm64':'arm64','x86_64':'amd64','amd64':'amd64'}[arch])
    subprocess.run(['go','build','-o',str(tmp/'auth'),'./tools/coexist-test-helper'],cwd=ROOT,env=env,check=True)
    listener = (ROOT/'internal/app/assets/rf-link-listener.hocon.tmpl').read_text() % (1884, json.dumps('http://rf-link-sas:8080/auth'))
    old_config = '''node { name = "emqx@127.0.0.1", cookie = "isolated-test-cookie", data_dir = "/opt/emqx/data" }
authentication = [{mechanism = password_based, backend = http, method = post, url = "http://fmo-sas:8080/auth", headers = {"content-type" = "application/json"}, body = {username = "${username}", password = "${password}"}}]
authorization { no_match = deny, sources = [{type = file, enable = true, path = "/opt/emqx/etc/test-acl.conf"}] }
'''
    (tmp/'emqx.conf').write_text(old_config + listener)
    (tmp/'acl.conf').write_text('{allow, all, all, ["room/#"]}.\n{deny, all}.\n')
    docker('network','create',PREFIX)
    try:
        for role, alias in [('fmo','fmo-sas'),('rflink','rf-link-sas')]:
            start(role,'--network-alias',alias,'--read-only','--cap-drop','ALL','--security-opt','no-new-privileges:true','-v',str(tmp/'auth')+':/test-auth:ro','--entrypoint','/test-auth',SAS,role)
        broker = start('broker','-p','127.0.0.1::1883','-p','127.0.0.1::1884','-v',str(tmp/'emqx.conf')+':/opt/emqx/etc/emqx.conf:ro','-v',str(tmp/'acl.conf')+':/opt/emqx/etc/test-acl.conf:ro',EMQX)
        old_port, new_port = port(broker,1883), port(broker,1884)
        deadline = time.monotonic()+90
        while True:
            try: connect(old_port,'fmo').close(); connect(new_port,'rflink').close(); break
            except (OSError,AssertionError):
                if time.monotonic()>deadline or docker('inspect','--format','{{.State.Running}}',broker)!='true': raise
                time.sleep(1)
        for p, role, other in [(old_port,'fmo','rflink'),(new_port,'rflink','fmo')]:
            connect(p,role).close()
            for wrong in ['', 'wrong', other]: connect(p,wrong,expected=5)
        print('PASS: positive, anonymous/wrong/cross-identity rejection on both listeners',flush=True)
        exchange(old_port,new_port)
        print('PASS: bidirectional cross-port messages and shared ACL rejection',flush=True)
        for role, blocked, good, good_role in [('rflink',new_port,old_port,'fmo'),('fmo',old_port,new_port,'rflink')]:
            docker('stop',PREFIX+'-'+role)
            connect(blocked,role,expected=5); connect(good,good_role).close()
            # Remove the stopped fixture and recreate only this synthetic backend.
            alias = 'rf-link-sas' if role=='rflink' else 'fmo-sas'
            docker('rm','-v',PREFIX+'-'+role)
            containers.remove(PREFIX+'-'+role)
            start(role,'--network-alias',alias,'--read-only','--cap-drop','ALL','-v',str(tmp/'auth')+':/test-auth:ro','--entrypoint','/test-auth',SAS,role)
            time.sleep(1)
        print('PASS: each authentication backend outage is isolated',flush=True)
        # Withdraw only the RF Link listener, retaining byte-for-byte original FMO configuration.
        (tmp/'emqx.conf').write_text(old_config)
        docker('restart',broker)
        old_port = port(broker,1883)  # Docker may reassign an ephemeral host port on restart.
        deadline=time.monotonic()+60
        while True:
            try: connect(old_port,'fmo').close(); break
            except (OSError,AssertionError):
                if time.monotonic()>deadline or docker('inspect','--format','{{.State.Running}}',broker)!='true': raise
                time.sleep(1)
        print('PASS: original FMO access remains after removing RF Link listener',flush=True)
    except BaseException:
        for name in containers:
            print(docker('logs','--tail','35',name),flush=True)
        raise
    finally:
        for name in reversed(containers): subprocess.run(['docker','rm','-fv',name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        docker('network','rm',PREFIX)
