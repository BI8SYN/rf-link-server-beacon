package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"github.com/BI8SYN/rf-link-server-beacon/internal/identity"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"strconv"
	"strings"
	"time"
)

// SASCommands is a read-only preview. It never acquires the running daemon's
// lock, renews credentials, applies SAS settings or requests a message signature.
func SASCommands(ctx context.Context, cfg config.Config, deployment string) (string, error) {
	switch deployment {
	case "full-stack", "compose", "docker", "native":
	default:
		return "", errors.New("deployment required: use ./beacon.sh sas-command for host detection, or --deployment full-stack|compose|docker|native")
	}
	if e := cfg.Validate(); e != nil {
		return "", e
	}
	var state State
	if e := (&Store{dir: cfg.StateDir}).Read(&state); e != nil {
		return "", e
	}
	if state.Schema != 1 || state.Origin != cfg.Signer.BaseURL || state.Callsign != cfg.Callsign || !signer.ValidID(state.BroadcastID) {
		return "", errors.New("no matching enrolled identity; wait for publication and credential delivery")
	}
	var token string
	if state.Credential != nil {
		token = state.Credential.MachineToken
	} else {
		var e error
		token, e = config.Secret(cfg.Signer.CredentialFile)
		if e != nil {
			return "", errors.New("identity has no machine credential; wait for automatic delivery")
		}
	}
	client, e := signer.New(cfg.Signer.BaseURL, token, cfg.Signer.AllowLoopbackHTTP)
	if e != nil {
		return "", e
	}
	info, e := client.Identity(ctx, state.BroadcastID)
	if e != nil {
		return "", e
	}
	if info.ServerID != state.ServerID || info.EnrollmentID != state.EnrollmentID {
		return "", errors.New("SAS identity ownership mismatch")
	}
	bound, e := info.Verify(cfg.Trust.Roots, time.Now(), cfg.Callsign, cfg.Station.CountryCode)
	if e != nil {
		return "", e
	}
	if state.CRLs == nil {
		state.CRLs = map[string]identity.Mark{}
	}
	preview := &Service{cfg: cfg, state: state, client: client, now: time.Now}
	if e = preview.checkCRLs(ctx, bound); e != nil {
		return "", e
	}
	uid := strconv.FormatUint(bound.User.UID, 10)
	issuer := strconv.FormatUint(bound.User.IssuerSN, 10)
	port := strconv.FormatUint(cfg.Station.Port, 10)
	fp := bound.User.Fingerprint()
	args := []string{"--server-uid", uid, "--server-callsign", bound.User.Callsign, "--mqtt-host", cfg.Station.Host, "--mqtt-port", port, "--cert-fingerprint", fp, "--issuer-sn", issuer, "--http-addr", "0.0.0.0", "--http-port", "8080"}
	rootNote := fmt.Sprintf("# 确认SAS已安装认可的Root（TBS指纹 %s），保留原信任根。\n", bound.RootFingerprint)
	switch deployment {
	case "full-stack":
		return fmt.Sprintf(`# 本项目完整服务器：将以下六项合并到.env，保留Dashboard密码。
SAS_SERVER_UID=%s
SAS_SERVER_CALLSIGN=%s
SAS_MQTT_HOST=%s
SAS_MQTT_PORT=%s
SAS_CERT_FINGERPRINT=%s
SAS_ISSUER_SN=%s
# 然后执行：docker compose up -d --wait
# 首次部署需在.env设置EMQX_DASHBOARD_PASSWORD；模板会安装公开根并配置认证。
`, uid, bound.User.Callsign, cfg.Station.Host, port, fp, issuer), nil
	case "compose":
		dockerArgs, _ := json.Marshal(args)
		return fmt.Sprintf(`# SAS Compose：把command合并到原SAS服务，保留镜像、卷和其他配置。
command: %s
# 合并后在原项目目录执行：docker compose up -d
# EMQX与SAS须在同一网络，认证URL为http://<SAS服务名>:8080/auth。
`, dockerArgs) + rootNote, nil
	case "docker":
		return "# 独立Docker SAS：把下列参数放到原docker run命令的镜像名之后，保留原卷和网络。\n" +
			strings.Join(args, " ") + "\n# 备份后用原卷重建SAS容器，EMQX认证URL指向实际SAS容器地址:8080/auth；不必发布公网8080。\n" + rootNote, nil
	default:
		return fmt.Sprintf(`# 原生SAS与EMQX同宿主机：手工运行时使用以下命令。
./sas --server-uid %s --server-callsign %s --mqtt-host %s --mqtt-port %s --cert-fingerprint %s --issuer-sn %s --http-addr 127.0.0.1 --http-port 8080
# 已有系统服务时，将参数合并到原服务配置后重启，不另外启动第二个SAS。
# EMQX认证URL：http://127.0.0.1:8080/auth；混合部署需按实际网络调整监听和回调。
`, uid, bound.User.Callsign, cfg.Station.Host, port, fp, issuer) + rootNote, nil
	}
}
