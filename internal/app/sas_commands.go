package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/config"
	"github.com/BI8SYN/rf-link-server-beacon/internal/identity"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"strconv"
	"time"
)

// SASCommands is a read-only preview. It never acquires the running daemon's
// lock, renews credentials, applies SAS settings or requests a message signature.
func SASCommands(ctx context.Context, cfg config.Config, deployment string, options SASSetupOptions) (string, error) {
	if err := ValidateSASSetup(options, deployment, cfg.Station.Port); err != nil {
		return "", err
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
	// Docker new-server modes all use the reviewed full stack; never reuse FMO state.
	if options.Scenario == "new-server" && deployment != "native" {
		deployment = "full-stack"
	}
	uid := strconv.FormatUint(bound.User.UID, 10)
	issuer := strconv.FormatUint(bound.User.IssuerSN, 10)
	port := strconv.FormatUint(cfg.Station.Port, 10)
	fp := bound.User.Fingerprint()
	rootNote := fmt.Sprintf("# 确认SAS已安装认可的Root（TBS指纹 %s），保留原信任根。\n", bound.RootFingerprint)
	if options.Scenario == "coexist" {
		return coexistCommands(deployment, uid, bound.User.Callsign, cfg.Station.Host, port, fp, issuer, rootNote), nil
	}
	if deployment == "full-stack" {
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
	}
	return fmt.Sprintf(`# 新建独立RF Link SAS，不修改现有服务。
mkdir -p "$HOME/.rf-link-sas/roots"
cp deploy/trust/roots/*.json "$HOME/.rf-link-sas/roots/"
./sas --config "$HOME/.rf-link-sas/config.json" --roots-dir "$HOME/.rf-link-sas/roots" --server-uid %s --server-callsign %s --mqtt-host %s --mqtt-port %s --cert-fingerprint %s --issuer-sn %s --http-addr 127.0.0.1 --http-port 8080
# 将该命令配置为systemd服务。MQTT认证回调：http://127.0.0.1:8080/auth。
# 同宿主机原生Broker可用上述回环地址；混合部署按实际网络调整受限监听和回调。
`, uid, bound.User.Callsign, cfg.Station.Host, port, fp, issuer) + rootNote, nil
}
