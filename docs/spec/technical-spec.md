---
last-reviewed: 2026-11-07
---
# 技术约束

- Go，module 为 github.com/BI8SYN/rf-link-server-beacon。当前 go.mod 1.24.0 是构建基线，生产必须选择当时仍受支持的工具链和固定 builder digest。
- Linux amd64/arm64，CGO=0；Go 标准库 net/http、net、crypto/ed25519。确定性CBOR使用内部受限编码器，只实现FMO所需的uint/text/bytes/bool/定长数组；解析CERT后重编码比较规范性。与上游公开向量逐字节比较。Ed25519签名验证由标准库执行，额外使用固定filippo.io/edwards25519 v1.1.0检查公钥规范编码、非单位元和素数阶子群。
- JSON严格解码、schema_version=1，拒绝未知字段、缺字段、重复键、null冒充标量、浮点/指数、超安全整数和非法Unicode，限制文件/响应长度；无默认生产呼号/地址。拒绝逗号、CR/LF、NUL、非法端口与越界 SSID。样例坐标仅示例，配置时必须主动确认。
- HTTPS 签名请求默认验证系统 TLS；FMO Root 信任库与 HTTPS 系统 CA 是两套概念。不得关闭 TLS 验证解决接入问题。
- 申请回执与机器凭据原子保存在0600状态文件，状态目录0700、配置0600；公开证书不含私钥。单data挂载不是对宿主机root的安全边界。fmba1只申请/查询/领取，fmb1只身份/签名。
- 签名服务生成/构造规范载荷，客户端只能提交结构化业务字段。用户证书 subject.publicKey 验证返回 SIG，匹配 expected identity 与本次完整输入，禁止只验证响应里自带的任意公钥。
- 公告配置由客户端保存，签名正文为requestId/station/online/peak，移除expectedProfileVersion；身份查询不含station/profileVersion。完整请求原子保存，超时或重启沿原快照重试；明确过期后才换新ID。旧profile待签请求切换时作废，原申请正文及全部身份/凭据保留。
- 同一节点单实例运行：状态锁防止多副本重复发报；远端限频为最终保障。建议请求超时 10s，最大响应 64KiB，重连指数退避 1–60s 带抖动；均需按实现与测试维护。
- 正常广播 600 秒一次；冷启动不追补历史广播；签名过期后不能重发缓存报文。600 秒桶允许当前±1由协议定义；发布者应只发送当前桶并留足传输余量。
- SIGTERM 取消网络与定时器，10 秒内退出。日志不写 token、Authorization、APRS passcode、私钥或完整身份载荷；公开证书也只记录指纹和序列号。
- CRL 需验签、签发者绑定、nextUpdate、防回滚；首版建议过期/未知状态暂停新广播，是否宽限需显式决策，不能静默 fail-open。
- 不做真实人数监测的第一版可使用明确标记的静态 0/0；不能伪造实时统计。若接 EMQX API，单独受限令牌，不共享 Dashboard 管理密码。

- sas-command部署识别在宿主机脚本完成，仅查询docker ps的镜像/Compose标签及本机SAS安装状态；不读取容器环境变量、不挂载socket。Go CLI仅接受明确部署类型，不把Beacon的容器环境作为SAS拓扑依据。

- SAS生成先校验scenario，仅new-server/coexist；coexist复用同SAS同端点，输出Mqtt.Clusters五字段（uid、callsign、mqtt_host、mqtt_port、certFingerprint）。离线候选生成用Python 3标准库，JSON键无歧义，保留原Server、管理员、Trust、HTTP及其他/未知字段；拒绝主UID/现存节点冲突、端点不匹配和覆盖已有文件，0600输出。管理员检查后手工应用到实际持久卷并重启原SAS；旧转换/新监听参数拒绝，CI不访问生产设施。

- 原生安装只下载固定GitHub仓库的Release与SHA256SUMS，HTTPS校验启用，支持Linux amd64/arm64；校验失败保留旧文件且不停止服务，不自动退回源码编译。GitHub CI使用CGO_ENABLED=0构建两架构，v* tag发布公开资产。

- Docker 管理模式复用身份申请、凭据、CRL 和签名快照契约。SAS 配置通过专用卷原子提交，公用 MQTT 端口只由 Beacon 转发，EMQX 每次启动重新验证 HTTP 认证回调。管理模式与原有独立 Beacon 接口分开，使用 GitHub Actions 构建 amd64/arm64 镜像。
