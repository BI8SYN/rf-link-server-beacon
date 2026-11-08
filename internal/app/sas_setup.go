package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

type SASSetupOptions struct{ Scenario string }

// ValidateSASSetup runs before any identity request.
func ValidateSASSetup(o SASSetupOptions, deployment string, _ uint64) error {
	switch deployment {
	case "full-stack", "compose", "docker", "native":
	default:
		return errors.New("deployment must be full-stack|compose|docker|native")
	}
	switch o.Scenario {
	case "new-server", "coexist":
	default:
		return errors.New("scenario required: new-server|coexist (convert retired)")
	}
	if deployment == "full-stack" && o.Scenario != "new-server" {
		return errors.New("full-stack only supports new-server; coexist uses the existing SAS")
	}
	return nil
}

func coexistCommands(deployment, uid, callsign, host, port, fp, issuer, rootNote string) string {
	id, _ := strconv.ParseUint(uid, 10, 64)
	p, _ := strconv.ParseUint(port, 10, 64)
	cluster, _ := json.Marshal(struct {
		UID         uint64 `json:"uid"`
		Callsign    string `json:"callsign"`
		Host        string `json:"mqtt_host"`
		Port        uint64 `json:"mqtt_port"`
		Fingerprint string `json:"certFingerprint"`
	}{id, callsign, host, p, fp})
	output := "# 已有FMO：保留原主身份、管理员及MQTT端口，向同一SAS的Mqtt.Clusters追加RF Link身份。\n# 在本仓库目录执行；需要Python 3。先替换实际配置路径，并备份原SAS配置及根目录。\numask 077\n"
	source := `"$SAS_CONFIG"`
	if deployment == "native" {
		output += "SAS_CONFIG='/替换为原SAS实际配置路径/config.json'\n"
	} else {
		output += "SAS_CONTAINER='替换为原SAS容器名'\nSAS_CONFIG='/替换为容器内实际配置路径/config.json'\n# 配置路径以原进程--config或启动日志为准，必须位于持久卷。\ndocker cp \"$SAS_CONTAINER:$SAS_CONFIG\" data/sas-config.original.json\nchmod 600 data/sas-config.original.json\n"
		source = "data/sas-config.original.json"
	}
	output += fmt.Sprintf("python3 tools/sas-config.py --source %s --output data/sas-config.rflink.json --cluster '%s'\n", source, cluster)
	output += "# 输出文件已存在时拒绝覆盖；保留旧候选/备份，改用新的输出文件名。\n"
	output += fmt.Sprintf("# 在原SAS的实际Trust.RootsDir加入deploy/trust/roots/rf-link-root-production-v1.json。\n# Trust.AllowIssuerSn非空时须明确允许RF Link签发者 %s，保留已有条目；不自动清空限制。\n", issuer) + rootNote
	output += "# 核对差异：仅Mqtt.Clusters追加以上五字段；确认原Server及其他策略保持。\n"
	if deployment == "native" {
		output += "diff -u \"$SAS_CONFIG\" data/sas-config.rflink.json\n# 确认后，将候选内容写回原文件（保留文件属主/权限），再重启原服务：\n# sudo sh -c 'cat data/sas-config.rflink.json > \"$1\"' sh \"$SAS_CONFIG\"\n# sudo systemctl restart 原SAS服务名\n"
	} else {
		output += "diff -u data/sas-config.original.json data/sas-config.rflink.json\n# 确认后，将候选内容写回原持久卷内配置，保留文件属主/权限：\n# docker exec -i \"$SAS_CONTAINER\" sh -c 'cat > \"$1\"' sh \"$SAS_CONFIG\" < data/sas-config.rflink.json\n"
		if deployment == "compose" {
			output += "# 在原SAS的Compose项目目录，仅重启原SAS服务：\n# docker compose restart 原SAS服务名\n"
		} else {
			output += "# docker restart \"$SAS_CONTAINER\"\n"
		}
	}
	return output + "# MQTT认证回调保持原样；两种身份共享根和认证策略。验证原FMO及RF Link客户端登录/语音后，再doctor/start。\n"
}
