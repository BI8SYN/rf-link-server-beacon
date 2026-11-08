#!/usr/bin/env python3
"""Prepare an additive SAS candidate offline; never apply it to a running service."""
import argparse
import base64
import copy
import json
import os
import pathlib
import re
import sys


def object_pairs(pairs):
    result = {}
    seen = set()
    for key, value in pairs:
        folded = key.casefold()
        if folded in seen:
            raise ValueError('duplicate or ambiguous JSON key')
        seen.add(folded)
        result[key] = value
    return result


def load_json(text):
    return json.loads(text, object_pairs_hook=object_pairs,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError('invalid JSON number')))


def key(obj, name, optional=False):
    if not isinstance(obj, dict):
        raise ValueError('expected SAS configuration object')
    matches = [k for k in obj if k.casefold() == name.casefold()]
    if len(matches) != 1:
        if not matches and optional:
            return name
        raise ValueError('missing or ambiguous SAS field: ' + name)
    return matches[0]


def positive_int(value, maximum):
    return type(value) is int and 0 < value <= maximum


def merge(document, cluster):
    # Validate case-insensitive key uniqueness even when called without load_json.
    result = load_json(json.dumps(document, allow_nan=False))
    if not isinstance(cluster, dict) or set(cluster) != {'uid', 'callsign', 'mqtt_host', 'mqtt_port', 'certFingerprint'}:
        raise ValueError('invalid Cluster fields')
    if not positive_int(cluster['uid'], 2**53-1) or not positive_int(cluster['mqtt_port'], 65535):
        raise ValueError('invalid Cluster UID or port')
    if not isinstance(cluster['callsign'], str) or not re.fullmatch(r'[A-Z0-9]{3,10}', cluster['callsign']):
        raise ValueError('invalid Cluster callsign')
    if not isinstance(cluster['mqtt_host'], str) or not re.fullmatch(r'[A-Za-z0-9.:[\]-]+', cluster['mqtt_host']):
        raise ValueError('invalid Cluster host')
    fp = cluster['certFingerprint']
    if not isinstance(fp, str) or not re.fullmatch(r'[A-Za-z0-9_-]{43}', fp):
        raise ValueError('invalid Cluster fingerprint')
    if base64.urlsafe_b64encode(base64.urlsafe_b64decode(fp+'=')).decode().rstrip('=') != fp:
        raise ValueError('noncanonical Cluster fingerprint')
    server = result[key(result, 'Server')]
    main_uid = server[key(server, 'Uid')]
    if not positive_int(main_uid, 2**53-1) or main_uid == cluster['uid']:
        raise ValueError('invalid or conflicting primary UID; preserve the FMO identity')
    if server[key(server, 'CertFingerprint')] == fp:
        raise ValueError('fingerprint conflicts with the primary identity')
    mqtt = result[key(result, 'Mqtt')]
    host, port = mqtt[key(mqtt, 'Host')], mqtt[key(mqtt, 'Port')]
    if not isinstance(host, str) or host.casefold() != cluster['mqtt_host'].casefold() or type(port) is not int or port != cluster['mqtt_port']:
        raise ValueError('broadcast endpoint must match the existing FMO MQTT host and port')
    clusters_key = key(mqtt, 'Clusters', optional=True)
    clusters = mqtt.setdefault(clusters_key, [])
    if not isinstance(clusters, list):
        raise ValueError('Mqtt.Clusters must be an array')
    identical = 0
    for entry in clusters:
        uid = entry[key(entry, 'uid')]
        fingerprint = entry[key(entry, 'certFingerprint')]
        if uid == cluster['uid'] or fingerprint == fp:
            if entry != cluster:
                raise ValueError('Cluster identity collision; inspect certificate rotation or UID conflict manually')
            identical += 1
    if identical > 1:
        raise ValueError('duplicate Cluster identities')
    if not identical:
        clusters.append(copy.deepcopy(cluster))
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--cluster', required=True, help='public Cluster JSON from verified sas-command')
    args = parser.parse_args()
    source, output = pathlib.Path(args.source), pathlib.Path(args.output)
    try:
        if source.resolve() == output.resolve():
            raise ValueError('output must be a new candidate file')
        document = load_json(source.read_text(encoding='utf-8'))
        candidate = merge(document, load_json(args.cluster))
        encoded = (json.dumps(candidate, ensure_ascii=False, indent=2, allow_nan=False)+'\n').encode()
        # O_EXCL refuses existing files and symlinks; no source or runtime mutations.
        fd = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        try:
            with os.fdopen(fd, 'wb') as stream:
                stream.write(encoded)
                stream.flush()
                os.fsync(stream.fileno())
        except BaseException:
            output.unlink()
            raise
    except (ValueError, OSError, TypeError, KeyError):
        print('SAS候选生成失败：检查配置结构、同地址端口、UID/指纹冲突和输出路径；原文件未修改。', file=sys.stderr)
        return 1
    print('SAS候选已生成；尚未应用。请备份并核对差异，仅Mqtt.Clusters新增身份。')
    return 0


if __name__ == '__main__':
    sys.exit(main())
