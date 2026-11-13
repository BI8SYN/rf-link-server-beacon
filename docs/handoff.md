---
last-reviewed: 2026-11-07
---
# 开发交接

先读 README → project-brief → spec → architecture → contracts → protocol。源码包含 config、identity、protocol、signer、aprs、app 和 cli；模块名与公开 GitHub 路径一致。

已实现：自助申请、领取/续领凭据、链/CRL 验证、受限代签、CERT 重试快照、本地组包、Docker 单目录与 systemd 部署入口、SAS 配置代码（宿主机识别实际部署并按类型输出）、独立 EMQX/SAS 完整栈 Compose。

协议与实际广播已验证；完整栈验证范围单独记录在 [公开验收记录](development/open-source-validation.md)。不得将容器 healthy 等同合法设备认证、语音通信或官方设备发现兼容性通过。原生 systemd 仅做隔离检查，宿主机安装仍待测试。

## 后续验收

- 使用管理员自己的完整身份配置服务器，验证合法 FMO / RF Link 客户端登录、目标 UID/指纹和语音收发。
- 验证官方设备的实际信任根与发现行为。
- 测试真实 Linux systemd 首次安装、升级和卸载。
- 明确实际部署的备份、升级、恢复及告警策略；远端托管与审批可用性由 Trust 运营方保障。

## 开发检查

```sh
go test -race ./...
go vet ./...
go mod verify
sh -n beacon.sh
# 完整栈检查先配置.env；CI使用合成参数。
docker compose config --quiet
docker compose -f compose.beacon.yaml config --quiet
```

CI 使用临时密钥和回环服务，禁止生产申请、签名及 APRS 发报。个人运维资料留在忽略的 data/，不能发布到源码或镜像。不要操作生产 CA；实际生产发报或服务变更需明确授权。

2026-11-06流程调整：优先简单教程，无需全自动应用SAS。首次接入使用setup/wait、sas-command配置、客户端验证、doctor/start四步；README不再先推荐身份就绪后自动发报的deploy-docker。脚本setup/wait成功提示下一步，仍不发报。

## 两种部署场景

仅保留new-server完全新建和coexist已有FMO。新建Docker使用完整EMQX/SAS栈；已有FMO保留原主身份和管理员，在原SAS的Mqtt.Clusters追加RF Link目标身份，共用原MQTT地址和端口，根及认证策略共享。

sas-command在兼容模式识别原SAS部署，仅输出对应类型的配置步骤。tools/sas-config.py离线生成候选，只追加一个Cluster，保留原配置其他字段；拒绝UID/指纹冲突、端点不匹配、歧义键和已有输出文件，不自动应用。管理员备份、安装认可根、按需扩展非空AllowIssuerSn、检查差异后仅重启原SAS。证书轮换只更新对应Cluster指纹，不能用独立SAS导出配置覆盖原FMO配置。

转换入口、双SAS Compose和新监听器生成已移除，旧实例不自动迁移或删除。隔离容器配置加载测试与真实客户端/语音验收是不同范围，后者须部署者自行验证。

2026-11-09：deploy-native改为自动下载GitHub Release的Linux amd64/arm64二进制并验证SHA256SUMS，无需Go。CI交叉构建artifact，v* tag发布Release；BEACON_VERSION可固定版本。源码开发BEACON_NATIVE=1 setup仍可本地编译。
