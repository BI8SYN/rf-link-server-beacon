#!/bin/sh
# Download public release assets only. Never compile or touch service/data here.
set -eu
umask 077
[ "$#" = 1 ] || exit 2
case "$(uname -m)" in
  x86_64|amd64) BEACON_ARCH=amd64 ;;
  aarch64|arm64) BEACON_ARCH=arm64 ;;
  *) printf '%s\n' 'Unsupported architecture; Linux amd64/arm64 required.' >&2; exit 2 ;;
esac
BEACON_RELEASE=${BEACON_VERSION:-latest}
case "$BEACON_RELEASE" in
  latest) BEACON_URL=https://github.com/BI8SYN/rf-link-server-beacon/releases/latest/download ;;
  v[0-9]*)
    case "$BEACON_RELEASE" in *[!A-Za-z0-9._-]*) printf '%s\n' 'Invalid release version.' >&2; exit 2;; esac
    BEACON_URL=https://github.com/BI8SYN/rf-link-server-beacon/releases/download/$BEACON_RELEASE ;;
  *) printf '%s\n' 'BEACON_VERSION must be latest or a v-prefixed release tag.' >&2; exit 2 ;;
esac
command -v curl >/dev/null || { printf '%s\n' 'Install curl and CA certificates first.' >&2; exit 1; }
[ ! -L "$1" ] || { printf '%s\n' 'Binary destination must not be a symlink.' >&2; exit 1; }
mkdir -p "$(dirname -- "$1")"
BEACON_TEMP=$(mktemp -d "$(dirname -- "$1")/.beacon-download.XXXXXXXX")
trap 'rm -rf "$BEACON_TEMP"' EXIT HUP INT TERM
BEACON_ASSET=rf-link-server-beacon_linux_$BEACON_ARCH
for BEACON_FILE in "$BEACON_ASSET" SHA256SUMS; do
  curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --tlsv1.2 --retry 3 --connect-timeout 15 --max-time 180 -o "$BEACON_TEMP/$BEACON_FILE" "$BEACON_URL/$BEACON_FILE"
done
BEACON_EXPECTED=$(awk -v name="$BEACON_ASSET" '$2 == name && NF == 2 { n++; hash=$1 } END { if (n == 1) print hash; else exit 1 }' "$BEACON_TEMP/SHA256SUMS")
[ "${#BEACON_EXPECTED}" = 64 ] || exit 1
case "$BEACON_EXPECTED" in *[!0-9a-f]*) exit 1;; esac
if command -v sha256sum >/dev/null; then
  BEACON_ACTUAL=$(sha256sum "$BEACON_TEMP/$BEACON_ASSET" | cut -d ' ' -f 1)
else
  BEACON_ACTUAL=$(shasum -a 256 "$BEACON_TEMP/$BEACON_ASSET" | cut -d ' ' -f 1)
fi
[ "$BEACON_EXPECTED" = "$BEACON_ACTUAL" ] || { printf '%s\n' 'Release checksum mismatch; existing binary preserved.' >&2; exit 1; }
chmod 0755 "$BEACON_TEMP/$BEACON_ASSET"
mv -f "$BEACON_TEMP/$BEACON_ASSET" "$1"
printf '%s\n' "Downloaded and verified Linux $BEACON_ARCH ($BEACON_RELEASE)."
