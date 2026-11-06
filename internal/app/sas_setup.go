package app

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type SASSetupOptions struct {
	Scenario    string
	FMOPort     uint64
	HTTPPort    uint64
	MQTTNetwork string
	AuthURL     string
}

func (o SASSetupOptions) fmoPort() uint64 {
	if o.FMOPort == 0 {
		return 1883
	}
	return o.FMOPort
}
func (o SASSetupOptions) httpPort() uint64 {
	if o.HTTPPort == 0 {
		return 18081
	}
	return o.HTTPPort
}

var networkName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$`)

// ValidateSASSetup runs before any identity request; port=0 validates CLI flags only.
func ValidateSASSetup(o SASSetupOptions, deployment string, port uint64) error {
	switch deployment {
	case "full-stack", "compose", "docker", "native":
	default:
		return errors.New("deployment must be full-stack|compose|docker|native")
	}
	switch o.Scenario {
	case "new-server", "coexist", "convert":
	default:
		return errors.New("scenario required: new-server|coexist|convert")
	}
	if deployment == "full-stack" && o.Scenario != "new-server" {
		return errors.New("full-stack only supports new-server; use a separate SAS for coexist or the existing SAS for convert")
	}
	if o.fmoPort() > 65535 || o.httpPort() > 65535 {
		return errors.New("invalid FMO or SAS HTTP port")
	}
	if o.Scenario != "coexist" && (o.FMOPort != 0 || o.HTTPPort != 0 || o.MQTTNetwork != "" || o.AuthURL != "") {
		return errors.New("FMO port, HTTP port, MQTT network and auth URL options only apply to coexist")
	}
	if o.Scenario == "coexist" && port != 0 && port == o.fmoPort() {
		return errors.New("coexist requires a new MQTT port; edit server.port in data/config.json (do not re-enroll), or specify the actual --fmo-port")
	}
	if o.MQTTNetwork != "" && (!networkName.MatchString(o.MQTTNetwork) || deployment == "native") {
		return errors.New("invalid Docker MQTT network or native/network combination")
	}
	if o.AuthURL != "" {
		u, e := url.Parse(o.AuthURL)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "/auth" || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(o.AuthURL, "\r\n\x00") {
			return errors.New("auth URL must be http(s)://host[:port]/auth without credentials, query or fragment")
		}
	}
	return nil
}

//go:embed assets/rf-link-listener.hocon.tmpl
var listenerTemplate string

// ListenerHOCON is an additive fragment, not a replacement Broker configuration.
func ListenerHOCON(port uint64, authURL string) string {
	encoded, _ := json.Marshal(authURL)
	return fmt.Sprintf(listenerTemplate, port, encoded)
}

func coexistCommands(o SASSetupOptions, deployment, uid, callsign, host, port, fp, issuer, rootNote string) string {
	authURL := o.AuthURL
	if authURL == "" {
		authURL = fmt.Sprintf("http://127.0.0.1:%d/auth", o.httpPort())
		if o.MQTTNetwork != "" {
			authURL = "http://rf-link-sas:8080/auth"
		}
	}
	var output string
	if deployment == "native" {
		output = fmt.Sprintf(`# 保留原FMO SAS、端口和身份；新建独立RF Link SAS。
# 在本仓库目录准备新的Root目录：
mkdir -p "$HOME/.rf-link-sas/roots"
cp deploy/trust/roots/*.json "$HOME/.rf-link-sas/roots/"
# 在SAS二进制目录运行，或将此命令作为新的独立系统服务（勿修改原FMO服务）：
./sas --config "$HOME/.rf-link-sas/config.json" --roots-dir "$HOME/.rf-link-sas/roots" --server-uid %s --server-callsign %s --mqtt-host %s --mqtt-port %s --cert-fingerprint %s --issuer-sn %s --http-addr 127.0.0.1 --http-port %d
# 原生SAS默认仅适用于同宿主机原生Broker。其他拓扑须配置受限可达监听，再显式指定--auth-url。
`, uid, callsign, host, port, fp, issuer, o.httpPort()) + rootNote
	} else {
		output = fmt.Sprintf(`# 保留原FMO服务；以下SAS字段保存到新文件.env.rflink，不覆盖原.env。
SAS_SERVER_UID=%s
SAS_SERVER_CALLSIGN=%s
SAS_MQTT_HOST=%s
SAS_MQTT_PORT=%s
SAS_CERT_FINGERPRINT=%s
SAS_ISSUER_SN=%s
RF_LINK_SAS_HTTP_PORT=%d
`, uid, callsign, host, port, fp, issuer, o.httpPort())
		if o.MQTTNetwork != "" {
			output += "RF_LINK_MQTT_NETWORK=" + o.MQTTNetwork + "\n# 启动新的RF Link SAS（加入原MQTT容器网络）：\n# docker compose --env-file .env.rflink -f compose.sas.yaml -f compose.sas-network.yaml up -d --wait\n"
		} else {
			output += "# 启动新的RF Link SAS（适用于同宿主机原生MQTT）：\n# docker compose --env-file .env.rflink -f compose.sas.yaml up -d --wait\n# MQTT在Docker内时，重新生成并加--mqtt-network 原Broker网络名。\n"
		}
	}
	return output + fmt.Sprintf("\n# 增量添加到原EMQX 5.8.9的配置；原FMO端口%d保持不变。\n", o.fmoPort()) + ListenerHOCON(parsePort(port), authURL) + "# Docker Broker还需在原Compose发布新端口 " + port + ":" + port + "；防火墙仅放行新MQTT端口。\n# 备份后应用增量配置，验证旧FMO和新RF Link登录、消息收发，再启用广播。\n"
}

func parsePort(s string) uint64 { p, _ := strconv.ParseUint(s, 10, 16); return p }
