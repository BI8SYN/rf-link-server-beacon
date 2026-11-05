#!/bin/sh
# Local deployment entry point. No remote script download, secrets in arguments,
# production signing or APRS transmission during setup.
set -eu
umask 077
BEACON_PROJECT_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
BEACON_ACTION=${1:-help}
if [ "$#" -gt 0 ]; then shift; fi
case "$BEACON_ACTION" in
  help|-h|--help)
    cat <<'USAGE'
Usage:
  ./beacon.sh deploy-docker --callsign BI8SYN --ssid 12 --grid OL15ic --name "RF Link" --host voice.example.com --owner "Owner" --email owner@example.com
  sudo ./beacon.sh deploy-native --callsign BI8SYN --ssid 12 --grid OL15ic --name "RF Link" --host voice.example.com --owner "Owner" --email owner@example.com
  ./beacon.sh setup --callsign BI8SYN --ssid 15 --grid FN31pr --name "RF Link" --host voice.example.com --port 1883
  ./beacon.sh status|wait|doctor|sas-command|once|sync-identity|start|stop|logs
Setup creates data/config.json, collects applicant name/email and submits directly.
Application receipts and approved machine credentials are managed automatically. It never sends APRS. Admin review and manual certificate
signing are required before doctor/once can succeed; start explicitly enables APRS.
Overrides: BEACON_DATA_DIR, BEACON_NATIVE=1 (local CLI), BEACON_DEV=1 (Docker host network).
USAGE
    exit 0 ;;
  deploy-docker|deploy-native|setup|init|enroll|status|wait|doctor|sas-command|once|sync-identity|start|stop|logs|version) ;;
  *) printf '%s\n' 'Unknown command; use ./beacon.sh help' >&2; exit 2 ;;
esac
if [ "$BEACON_ACTION" = deploy-native ]; then
  [ "$(uname -s)" = Linux ] && [ "$(id -u)" = 0 ] || {
    printf '%s\n' 'deploy-native requires Linux systemd and root; use sudo (macOS supports foreground native commands only).' >&2
    exit 2
  }
  command -v systemctl >/dev/null 2>&1 || { printf '%s\n' 'systemd is required.' >&2; exit 2; }
  [ "${BEACON_DATA_DIR:-/var/lib/rf-link-server-beacon}" = /var/lib/rf-link-server-beacon ] || {
    printf '%s\n' 'Native system service uses /var/lib/rf-link-server-beacon; move existing data there first.' >&2; exit 2;
  }
  BEACON_DATA_DIR=/var/lib/rf-link-server-beacon
  BEACON_NATIVE=1
fi
if [ "$BEACON_ACTION" = deploy-docker ]; then BEACON_NATIVE=0; fi
case "$BEACON_ACTION" in
  start|stop|logs|version)
    [ "$#" -eq 0 ] || { printf '%s\n' 'This command accepts no extra arguments.' >&2; exit 2; } ;;
esac
BEACON_DATA_DIR=${BEACON_DATA_DIR:-"$BEACON_PROJECT_ROOT/data"}
mkdir -p -- "$BEACON_DATA_DIR"
BEACON_DATA_DIR=$(CDPATH= cd -- "$BEACON_DATA_DIR" && pwd)
chmod 700 "$BEACON_DATA_DIR"
export BEACON_DATA_DIR
if [ "${BEACON_NATIVE:-0}" = 1 ]; then
  BEACON_PROGRAM_DATA=$BEACON_DATA_DIR
  BEACON_BINARY=${BEACON_BINARY:-"$BEACON_PROJECT_ROOT/bin/rf-link-server-beacon"}
else
  command -v docker >/dev/null 2>&1 || { printf '%s\n' 'Docker is required (or set BEACON_NATIVE=1).' >&2; exit 1; }
  BEACON_PROGRAM_DATA=/data
  BEACON_UID=${BEACON_UID:-$(id -u)}
  BEACON_GID=${BEACON_GID:-$(id -g)}
  if [ "$BEACON_UID" = 0 ]; then
    BEACON_UID=65532; BEACON_GID=65532
    chown "$BEACON_UID:$BEACON_GID" "$BEACON_DATA_DIR"
  fi
  export BEACON_UID BEACON_GID
fi
beacon_compose() {
  if [ "${BEACON_DEV:-0}" = 1 ]; then
    docker compose -f "$BEACON_PROJECT_ROOT/compose.beacon.yaml" -f "$BEACON_PROJECT_ROOT/compose.dev.yaml" "$@"
  else
    docker compose -f "$BEACON_PROJECT_ROOT/compose.beacon.yaml" "$@"
  fi
}
beacon_run() {
  if [ "${BEACON_NATIVE:-0}" = 1 ]; then
    [ -x "$BEACON_BINARY" ] || { printf '%s\n' 'Build the binary or run setup first.' >&2; return 1; }
    "$BEACON_BINARY" "$@"
  else
    beacon_compose run --rm --no-deps -T beacon "$@"
  fi
}
beacon_build() {
  if [ "${BEACON_NATIVE:-0}" = 1 ]; then
    mkdir -p -- "$(dirname -- "$BEACON_BINARY")"
    (cd "$BEACON_PROJECT_ROOT" && go build -trimpath -o "$BEACON_BINARY" ./cmd/rf-link-server-beacon)
  else
    beacon_compose build beacon
  fi
}
case "$BEACON_ACTION" in
  deploy-docker|deploy-native)
    if [ -f "$BEACON_DATA_DIR/config.json" ] && [ "$#" -gt 0 ]; then
      printf '%s\n' 'Existing configuration preserved; omit init arguments and edit the saved config explicitly.' >&2
      exit 2
    fi
    for BEACON_ARGUMENT in "$@"; do
      if [ "$BEACON_ARGUMENT" = --dev ]; then BEACON_DEV=1; fi
    done
    beacon_build
    if [ ! -f "$BEACON_DATA_DIR/config.json" ]; then
      beacon_run init --data-dir "$BEACON_PROGRAM_DATA" "$@"
    fi
    if [ "$BEACON_ACTION" = deploy-native ]; then
      "$BEACON_PROJECT_ROOT/deploy/systemd/install.sh" "$BEACON_BINARY"
    else
      beacon_compose up -d beacon
    fi
    printf '%s\n' 'Background service enabled. New identities wait for manual approval/signing; approved identities broadcast every 600 seconds. SAS candidates are exported locally, never applied to an external server.'
    ;;
  setup|init)
    for BEACON_ARGUMENT in "$@"; do
      if [ "$BEACON_ARGUMENT" = --dev ]; then BEACON_DEV=1; fi
    done
    beacon_build
    beacon_run init --data-dir "$BEACON_PROGRAM_DATA" "$@"
    if [ "$BEACON_ACTION" = setup ]; then
      beacon_run enroll --config "$BEACON_PROGRAM_DATA/config.json"
      printf '%s\n' 'Application submitted; wait for administrator review, manual certificate signing and publication. Use ./beacon.sh wait for automatic credential delivery. No APRS sent.'
    fi ;;
  enroll|status|wait|doctor|sas-command|once) beacon_run "$BEACON_ACTION" --config "$BEACON_PROGRAM_DATA/config.json" "$@" ;;
  sync-identity) beacon_run sync-identity --config "$BEACON_PROGRAM_DATA/config.json" --sas-output "$BEACON_PROGRAM_DATA/sas-config-$(date -u +%Y%m%dT%H%M%SZ).json" "$@" ;;
  version) beacon_run version ;;
  start)
    if [ "${BEACON_NATIVE:-0}" = 1 ]; then
      exec "$BEACON_BINARY" run --config "$BEACON_PROGRAM_DATA/config.json" --send "$@"
    else
      beacon_compose up -d beacon
    fi ;;
  stop|logs)
    if [ "${BEACON_NATIVE:-0}" = 1 ]; then
      printf '%s\n' 'Native mode runs in the foreground; use its terminal for logs and Ctrl-C to stop.' >&2
      exit 2
    fi
    if [ "$BEACON_ACTION" = stop ]; then beacon_compose stop beacon; else beacon_compose logs -f beacon; fi ;;
esac
