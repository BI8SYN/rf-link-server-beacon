#!/bin/sh
set -eu
umask 077
[ "$(id -u)" = 0 ] && [ "$#" = 1 ] || { printf '%s\n' 'Run deploy-native with sudo.' >&2; exit 2; }
BEACON_UNIT_SOURCE=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
BEACON_NATIVE_DATA=/var/lib/rf-link-server-beacon
[ ! -L "$BEACON_NATIVE_DATA" ] || { printf '%s\n' 'Native data directory must not be a symlink.' >&2; exit 2; }
[ -f "$BEACON_NATIVE_DATA/config.json" ] && [ -x "$1" ] || { printf '%s\n' 'Initialized data and built program required.' >&2; exit 2; }
if ! getent passwd rf-link-beacon >/dev/null; then
  useradd --system --user-group --home-dir "$BEACON_NATIVE_DATA" --no-create-home --shell /usr/sbin/nologin rf-link-beacon
fi
if [ -f /etc/systemd/system/rf-link-server-beacon.service ]; then
  systemctl stop rf-link-server-beacon.service
fi
install -d -m 0755 /usr/local/lib/rf-link-server-beacon
install -m 0755 "$1" /usr/local/lib/rf-link-server-beacon/beacon
install -m 0644 "$BEACON_UNIT_SOURCE/rf-link-server-beacon.service" /etc/systemd/system/rf-link-server-beacon.service
chown -R rf-link-beacon:rf-link-beacon "$BEACON_NATIVE_DATA"
chmod 0700 "$BEACON_NATIVE_DATA"
systemctl daemon-reload
systemctl enable --now rf-link-server-beacon.service
