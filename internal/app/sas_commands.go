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
	"time"
)

// SASCommands is a read-only preview. It never acquires the running daemon's
// lock, renews credentials, applies SAS settings or requests a message signature.
func SASCommands(ctx context.Context, cfg config.Config) (string, error) {
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
	dockerArgs, _ := json.Marshal(args)
	return fmt.Sprintf(`# 仅生成代码，未应用SAS。UID/证书指纹已验链及CRL；不含调用凭据。
# 原生SAS与EMQX同宿主机：在SAS二进制目录初始化/启动。
./sas --server-uid %s --server-callsign %s --mqtt-host %s --mqtt-port %s --cert-fingerprint %s --issuer-sn %s --http-addr 127.0.0.1 --http-port 8080
# EMQX认证URL：http://127.0.0.1:8080/auth

# 官方SAS Docker Compose：合并以下command到既有sas服务，保留原镜像/持久卷。
services:
  sas:
    command: %s
# sas与EMQX须在同一Docker网络；认证URL：http://sas:8080/auth。
# 0.0.0.0是容器内监听，不必向公网发布8080。

# 本项目完整服务器Compose的.env；官方Compose可使用前五项：
SAS_SERVER_UID=%s
SAS_SERVER_CALLSIGN=%s
SAS_MQTT_HOST=%s
SAS_MQTT_PORT=%s
SAS_CERT_FINGERPRINT=%s
SAS_ISSUER_SN=%s
# 保留其他配置；本项目Compose自动使用issuerSn并监听容器内0.0.0.0。

# 用户证书指纹不是根证书指纹。SAS还须安装已认可Root，TBS指纹：%s
# 原生默认roots为运行用户的~/.sas/roots；官方容器为/home/app/.sas/roots。
# 已有SAS先备份并合并实际持久配置，再重启原服务，不启动第二个监听进程。
# 若SAS与EMQX混用宿主机/容器，另按实际网络设置监听与回调，不能照搬localhost。
`, uid, bound.User.Callsign, cfg.Station.Host, port, fp, issuer, dockerArgs, uid, bound.User.Callsign, cfg.Station.Host, port, fp, issuer, bound.RootFingerprint), nil
}
