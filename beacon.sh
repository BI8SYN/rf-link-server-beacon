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
首次Docker接入（先替换YOURCALL及服务器资料）：
  ./beacon.sh setup --callsign YOURCALL --ssid 12 --grid OL15ic --name "RF Link" --host mqtt.example.com --owner "负责人" --email owner@example.com
  ./beacon.sh wait
  ./beacon.sh sas-command   # 选择：新建 / 保留FMO增加入口 / 转换
  按输出配置SAS，验证客户端能登录和收发语音后：
  ./beacon.sh doctor
  ./beacon.sh start

管理：./beacon.sh status|logs|stop
setup和wait不发广播；start才启用每600秒广播。
从零建服见docs/operations/full-server.md。

其他入口（身份就绪后自动广播，不自动应用SAS）：
  ./beacon.sh deploy-docker [初始化参数]
  sudo ./beacon.sh deploy-native [初始化参数]
覆盖选项：BEACON_DATA_DIR、BEACON_NATIVE=1（本地CLI）、BEACON_DEV=1。
USAGE
    exit 0 ;;
  deploy-docker|deploy-native|setup|init|enroll|status|wait|doctor|sas-command|once|sync-identity|start|stop|logs|version) ;;
  *) printf '%s\n' 'Unknown command; use ./beacon.sh help' >&2; exit 2 ;;
esac
# Detection stays on the host: never give the Beacon container Docker access.
if [ "$BEACON_ACTION" = sas-command ]; then
  BEACON_SAS_MODE=auto
  BEACON_SCENARIO=
  BEACON_FMO_PORT=0
  BEACON_SAS_HTTP_PORT=0
  BEACON_MQTT_NETWORK=
  BEACON_SAS_AUTH_URL=
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --scenario|--fmo-port|--sas-http-port|--mqtt-network|--auth-url)
        [ "$#" -ge 2 ] || { printf '%s\n' "Missing value: $1" >&2; exit 2; }
        case "$1" in
          --scenario) BEACON_SCENARIO=$2 ;;
          --fmo-port) BEACON_FMO_PORT=$2 ;;
          --sas-http-port) BEACON_SAS_HTTP_PORT=$2 ;;
          --mqtt-network) BEACON_MQTT_NETWORK=$2 ;;
          --auth-url) BEACON_SAS_AUTH_URL=$2 ;;
        esac
        shift 2 ;;
      --scenario=*) BEACON_SCENARIO=${1#*=}; shift ;;
      --deployment)
        [ "$#" -ge 2 ] || { printf '%s\n' '--deployment requires a value' >&2; exit 2; }
        BEACON_SAS_MODE=$2; shift 2 ;;
      --deployment=*) BEACON_SAS_MODE=${1#*=}; shift ;;
      -h|--help)
        printf '%s\n' '用法：./beacon.sh sas-command --scenario new-server|coexist|convert [--deployment auto|full-stack|compose|docker|native]' 'coexist可指定--fmo-port、--sas-http-port、--mqtt-network或--auth-url。' '不带场景时由终端选择；非交互脚本须明确指定。'
        exit 0 ;;
      *) printf '%s\n' '无效参数，见 ./beacon.sh sas-command --help' >&2; exit 2 ;;
    esac
  done
  if [ -z "$BEACON_SCENARIO" ]; then
    [ -t 0 ] || { printf '%s\n' '必须明确--scenario new-server|coexist|convert，不能根据原SAS猜测是否替换FMO身份。' >&2; exit 2; }
    printf '%s\n' '请选择目标：' '  1) 新建RF Link服务器' '  2) 保留FMO，增加RF Link入口' '  3) 将FMO转换为RF Link（先备份，不保留原FMO身份）' >&2
    printf '输入1–3：' >&2
    IFS= read -r BEACON_SCENARIO_SELECTION
    case "$BEACON_SCENARIO_SELECTION" in
      1) BEACON_SCENARIO=new-server ;;
      2) BEACON_SCENARIO=coexist ;;
      3) BEACON_SCENARIO=convert ;;
      *) printf '%s\n' '无效场景。' >&2; exit 2 ;;
    esac
  fi
  case "$BEACON_SCENARIO" in new-server|coexist|convert) ;; *) printf '%s\n' '无效--scenario。' >&2; exit 2 ;; esac
  # Creating a new SAS must not be inferred from the existing FMO container.
  if [ "$BEACON_SAS_MODE" = auto ] && [ "$BEACON_SCENARIO" != convert ]; then
    if command -v docker >/dev/null 2>&1; then
      if [ "$BEACON_SCENARIO" = coexist ]; then BEACON_SAS_MODE=compose; else BEACON_SAS_MODE=full-stack; fi
    else
      BEACON_SAS_MODE=native
    fi
  fi
  case "$BEACON_SAS_MODE" in
    auto|full-stack|compose|docker|native) ;;
    *) printf '%s\n' '无效--deployment；请选择full-stack、compose、docker或native。' >&2; exit 2 ;;
  esac
  if [ "$BEACON_SAS_MODE" = auto ]; then
    BEACON_SAS_CHOICES=' '
    BEACON_SAS_CONTAINERS=
    if command -v docker >/dev/null 2>&1; then
      BEACON_SAS_CONTAINERS=$(docker ps --format '{{.Image}}|{{.Label "com.docker.compose.service"}}|{{.Label "com.docker.compose.project.working_dir"}}|{{.Label "com.docker.compose.project.config_files"}}' 2>/dev/null) || BEACON_SAS_CONTAINERS=
    fi
    while IFS='|' read -r BEACON_SAS_IMAGE BEACON_SAS_SERVICE BEACON_SAS_WORKDIR BEACON_SAS_FILES; do
      case "$BEACON_SAS_IMAGE" in
        ghcr.io/bg5esn/fmo-server-authrozier-service:*|ghcr.io/bg5esn/fmo-server-authrozier-service@*)
          if [ -z "$BEACON_SAS_SERVICE" ]; then
            BEACON_SAS_CANDIDATE=docker
          elif [ "$BEACON_SAS_SERVICE" = sas ] && [ "$BEACON_SAS_WORKDIR" = "$BEACON_PROJECT_ROOT" ] && [ "$BEACON_SAS_FILES" = "$BEACON_PROJECT_ROOT/compose.yaml" ]; then
            BEACON_SAS_CANDIDATE=full-stack
          else
            BEACON_SAS_CANDIDATE=compose
          fi
          case "$BEACON_SAS_CHOICES" in
            *" $BEACON_SAS_CANDIDATE "*) ;;
            *) BEACON_SAS_CHOICES="$BEACON_SAS_CHOICES$BEACON_SAS_CANDIDATE " ;;
          esac ;;
      esac
    done <<SAS_CONTAINERS
$BEACON_SAS_CONTAINERS
SAS_CONTAINERS
    if command -v sas >/dev/null 2>&1 || [ -x ./sas ] || {
      command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet fmo-sas 2>/dev/null;
    }; then
      BEACON_SAS_CHOICES="${BEACON_SAS_CHOICES}native "
    fi
    case "$BEACON_SAS_CHOICES" in
      ' full-stack ') BEACON_SAS_MODE=full-stack ;;
      ' compose ') BEACON_SAS_MODE=compose ;;
      ' docker ') BEACON_SAS_MODE=docker ;;
      ' native ') BEACON_SAS_MODE=native ;;
      *)
        if [ ! -t 0 ]; then
          printf '%s\n' '无法唯一识别SAS部署（可能尚未安装、运行在其他主机或存在多种部署）。请显式使用--deployment full-stack|compose|docker|native。' >&2
          exit 2
        fi
        printf '%s\n' '请选择要配置的SAS：' '  1) 本项目完整服务器' '  2) 其他Docker Compose' '  3) 独立Docker容器' '  4) 原生SAS' >&2
        printf '输入1–4：' >&2
        IFS= read -r BEACON_SAS_SELECTION
        case "$BEACON_SAS_SELECTION" in
          1) BEACON_SAS_MODE=full-stack ;;
          2) BEACON_SAS_MODE=compose ;;
          3) BEACON_SAS_MODE=docker ;;
          4) BEACON_SAS_MODE=native ;;
          *) printf '%s\n' '无效选择；也可使用--deployment明确指定。' >&2; exit 2 ;;
        esac ;;
    esac
  fi
  set -- --deployment "$BEACON_SAS_MODE" --scenario "$BEACON_SCENARIO"
  if [ "$BEACON_FMO_PORT" != 0 ]; then set -- "$@" --fmo-port "$BEACON_FMO_PORT"; fi
  if [ "$BEACON_SAS_HTTP_PORT" != 0 ]; then set -- "$@" --sas-http-port "$BEACON_SAS_HTTP_PORT"; fi
  if [ -n "$BEACON_MQTT_NETWORK" ]; then set -- "$@" --mqtt-network "$BEACON_MQTT_NETWORK"; fi
  if [ -n "$BEACON_SAS_AUTH_URL" ]; then set -- "$@" --auth-url "$BEACON_SAS_AUTH_URL"; fi
fi
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
      printf '%s\n' '申请已提交，尚未广播。下一步：./beacon.sh wait' '等待管理员审核、人工签发和发布后，程序会自动领取凭据。'
    fi ;;
  wait)
    beacon_run wait --config "$BEACON_PROGRAM_DATA/config.json" "$@"
    printf '%s\n' '身份已就绪，尚未广播。下一步：./beacon.sh sas-command，按提示配置SAS；确认客户端能登录后再运行 ./beacon.sh doctor 和 ./beacon.sh start。' >&2 ;;
  enroll|status|doctor|sas-command|once) beacon_run "$BEACON_ACTION" --config "$BEACON_PROGRAM_DATA/config.json" "$@" ;;
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
