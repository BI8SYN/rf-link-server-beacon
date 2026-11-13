import hashlib
import os
import pathlib
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parent.parent

class DownloadTest(unittest.TestCase):
    def test_download_and_fail_closed(self):
        for arch in ('x86_64', 'aarch64', 'unsupported'):
            with self.subTest(arch=arch), tempfile.TemporaryDirectory() as tmp:
                tmp=pathlib.Path(tmp)
                tools=tmp/'tools'; tools.mkdir()
                asset=b'#!/bin/sh\necho test-version\n'
                (tmp/'asset').write_bytes(asset)
                name='rf-link-server-beacon_linux_'+('amd64' if arch=='x86_64' else 'arm64')
                (tmp/'SHA256SUMS').write_text(hashlib.sha256(asset).hexdigest()+'  '+name+'\n')
                (tools/'uname').write_text('#!/bin/sh\necho '+arch+'\n')
                (tools/'curl').write_text('''#!/bin/sh
while [ "$#" -gt 0 ]; do
 case "$1" in -o) out=$2; shift 2;; *) url=$1; shift;; esac
done
case "$url" in */SHA256SUMS) cp "$FIXTURE/SHA256SUMS" "$out";; *) cp "$FIXTURE/asset" "$out";; esac
''')
                for p in tools.iterdir(): p.chmod(0o755)
                target=tmp/'beacon'; target.write_text('old')
                env=dict(os.environ,PATH=str(tools)+':'+os.environ['PATH'],FIXTURE=str(tmp))
                cmd=['sh',str(ROOT/'deploy/native/download.sh'),str(target)]
                result=subprocess.run(cmd,env=env,capture_output=True)
                if arch=='unsupported':
                    self.assertNotEqual(result.returncode,0); self.assertEqual(target.read_text(),'old'); continue
                self.assertEqual(result.returncode,0,result.stderr)
                self.assertEqual(target.read_bytes(),asset)
                (tmp/'asset').write_text('tampered')
                self.assertNotEqual(subprocess.run(cmd,env=env,capture_output=True).returncode,0)
                self.assertEqual(target.read_bytes(),asset)
                env['BEACON_VERSION']='../../bad'
                self.assertNotEqual(subprocess.run(cmd,env=env,capture_output=True).returncode,0)

if __name__=='__main__': unittest.main()
