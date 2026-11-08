"""Offline preservation and collision tests; all identities are synthetic."""
import copy
import importlib.util
import json
import pathlib
import subprocess
import tempfile
import unittest

SCRIPT = pathlib.Path(__file__).with_name('sas-config.py')
spec = importlib.util.spec_from_file_location('sas_config', SCRIPT)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class MergeTest(unittest.TestCase):
    def setUp(self):
        self.cluster = dict(uid=20, callsign='AB1CD', mqtt_host='mqtt.example.com',
                            mqtt_port=1883, certFingerprint='A'*43)
        self.original = {
            'Server': {'Uid': 10, 'Callsign': 'AB1CD', 'CertFingerprint': 'B'*43,
                       'Admins': [{'uid': 11, 'role': 'admin'}]},
            'Mqtt': {'Host': 'mqtt.example.com', 'Port': 1883, 'Clusters': []},
            'Trust': {'RootsDir': '/original/roots', 'AllowIssuerSn': [9]},
            'Http': {'Addr': '127.0.0.1', 'Port': 8080},
            'FutureField': {'opaque': 9007199254740993}}

    def test_only_cluster_changes_and_repeat_is_idempotent(self):
        before = copy.deepcopy(self.original)
        result = module.merge(self.original, self.cluster)
        expected = copy.deepcopy(before)
        expected['Mqtt']['Clusters'].append(self.cluster)
        self.assertEqual(result, expected)
        self.assertEqual(self.original, before)
        self.assertEqual(module.merge(result, self.cluster), result)

    def test_existing_entries_and_camel_case_preserved(self):
        doc = {k[0].lower()+k[1:]: v for k, v in self.original.items()}
        doc['server']['uid'] = doc['server'].pop('Uid')
        doc['mqtt'] = {'host': 'mqtt.example.com', 'port': 1883,
                       'clusters': [dict(self.cluster, uid=30, certFingerprint='C'*42+'A')]}
        result = module.merge(doc, self.cluster)
        self.assertEqual(result['mqtt']['clusters'], [dict(self.cluster, uid=30, certFingerprint='C'*42+'A'), self.cluster])
        self.assertNotIn('Mqtt', result)

    def test_conflicts_do_not_replace_identity(self):
        for change in [dict(uid=10), dict(mqtt_port=1884), dict(mqtt_host='other.invalid'),
                       dict(uid=True), dict(uid=-1), dict(certFingerprint='B'*43), dict(certFingerprint='bad'), dict(extra=1)]:
            with self.subTest(change=change), self.assertRaises(ValueError):
                module.merge(self.original, dict(self.cluster, **change))
        for entry in [dict(self.cluster, certFingerprint='B'*43), dict(self.cluster, uid=30, certFingerprint='A'*43)]:
            # Same fingerprint on a different UID is also suspicious.
            doc = copy.deepcopy(self.original)
            doc['Mqtt']['Clusters'] = [entry]
            with self.assertRaises(ValueError):
                module.merge(doc, self.cluster)

    def test_invalid_structures_and_ambiguous_keys(self):
        for patch in [{'Mqtt': None}, {'Mqtt': {'Host':'mqtt.example.com','Port':1883,'Clusters':None}},
                      {'mqtt': self.original['Mqtt']}, {'Server': {}},
                      {'Mqtt': {'Host':'mqtt.example.com','host':'other','Port':1883,'Clusters':[]}}]:
            with self.subTest(patch=patch), self.assertRaises(ValueError):
                module.merge(dict(self.original, **patch), self.cluster)
        with self.assertRaises(ValueError):
            module.load_json('{"Mqtt": {}, "Mqtt": {}}')

    def test_cli_never_overwrites_source_or_existing_output(self):
        with tempfile.TemporaryDirectory() as tmp:
            source = pathlib.Path(tmp)/'original.json'
            output = pathlib.Path(tmp)/'candidate.json'
            content = json.dumps(self.original).encode()
            source.write_bytes(content)
            args = ['python3', str(SCRIPT), '--source', str(source), '--output', str(output),
                    '--cluster', json.dumps(self.cluster)]
            result = subprocess.run(args, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(source.read_bytes(), content)
            self.assertEqual(output.stat().st_mode & 0o777, 0o600)
            candidate = output.read_bytes()
            self.assertNotEqual(subprocess.run(args, capture_output=True).returncode, 0)
            self.assertEqual(output.read_bytes(), candidate)
            self.assertNotEqual(subprocess.run(args[:6]+[str(source)]+args[7:], capture_output=True).returncode, 0)
            self.assertEqual(source.read_bytes(), content)


if __name__ == '__main__':
    unittest.main()
