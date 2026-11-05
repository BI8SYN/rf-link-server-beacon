---
last-reviewed: 2026-11-05
---
# 开源首版验收

本次整理面向其他管理员的安装入口、完整服务器 Compose、官方教程引用和 SAS 配置代码；未操作现有生产 MQTT / SAS、CA 或生产广播身份。

## 已执行

- `go test -race ./...`、`go vet ./...`：全部通过。测试覆盖临时证书链、CRL、公开历史向量、申请与凭据恢复、原 CERT 快照、签名篡改/过期、无空格组包、回环 APRS 和部署脚本。
- `go mod verify`、shell 语法、独立/开发 Compose、完整栈合成参数的 `compose config --quiet`：通过。
- 实际本机 Docker 构建：固定 Go 1.27.3；镜像内全部 Go 测试与静态构建通过；最终 scratch 容器在 `--network none` 下输出版本。
- 完整栈使用独立项目、命名卷、合成 AB1CD / UID20 及回环 31883/38083 端口。SAS 1.0.6 和 EMQX 5.8.9 按 Compose 固定摘要运行，`up -d --wait` 通过。
- SAS 日志确认加载两份公开根；持久配置核对 UID、基础呼号、指纹、issuerSn、MQTT 地址、0.0.0.0:8080 及 `Update.Enabled=false`。SAS 重启后配置保留。
- 实际 MQTT CONNECT：匿名、错误凭据均收到 CONNACK 0x05；停止 SAS 后错误凭据仍被拒绝，恢复 SAS 后重新 healthy。
- 结构检查确认 SAS 不发布端口，Dashboard 只绑定回环，根初始化容器无网络。EMQX 健康检查使用其 Dashboard `/status`，覆盖实际 HTTP 服务可达性。
- `sas-command` 测试确认六项 `.env` 字段、完整原生与 Compose 参数；守护进程持锁时仍可只读生成，不改状态、不请求签名或泄漏凭据。

## 尚需部署者实际验收

合成身份不能用于真实登录；本轮未测试实际合法客户端、语音收发、FAS 审计或官方设备显示。原生 systemd 单位文件此前仅隔离检查，未在宿主机实际安装。完整栈应先用自己的已发布身份与合法客户端验证，再启用广播。

此前广播服务已完成真实 APRS 独立回传、严格 App 格式及 Docker 常驻恢复验证，但这不替代新服务器的认证与语音验收。个人原始运维资料保留在本地忽略目录，不属于开源交付。
