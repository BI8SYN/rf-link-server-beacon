---
last-reviewed: 2026-11-13
---
# Docker 一键新建 RF Link 服务器

准备 Docker 和 Compose v2，以及客户端可达的 MQTT 公网地址。在对应发行版下载并解压 `rf-link-server-compose.zip`。在解压目录复制样例并填写自己的资料。

```sh
cp .env.example .env
```

填写呼号、SSID、网格、服务器名称、MQTT 地址、申请联系人和邮箱。端口默认 1883，公网地址不要写容器名。

```sh
docker compose up -d
docker compose logs -f beacon
```

Beacon 自动提交申请。看到 `pending_review` 时，按日志里的 `enrollmentId` 到 [Trust 后台](https://trust.bi8syn.com/#enrollments) 审核、签发并发布。退出日志查看后服务继续在后台等待。发布后程序自动领取和验证身份，配置 SAS，检查 MQTT 认证，再开放 MQTT 并开始广播。

| 阶段 | 含义 |
|---|---|
| `submitting_application` | 正在提交身份申请 |
| `pending_review`、`awaiting_signature`、`publishing` | 等待 Trust 审核、签发或发布 |
| `waiting_mqtt_authentication` | 等待 SAS 加载配置，或等待 EMQX 认证回调通过探测 |
| `sending_first_announcement` | 正在准备首次签名和发报 |
| `broadcasting` | MQTT 已就绪，广播调度已启用 |
| `retrying`、`broadcast_retry` | 查看 `reason` 和 `retryAt` |
| `failed` | 根据 `reason` 检查配置或身份状态 |

```sh
docker compose exec beacon /rf-link-server-beacon status
docker compose logs --tail 50 sas emqx
```

`lastPublished` 为最近一次成功发报时间，`nextBroadcastAt` 为下一次广播的最早时间。时间字段使用 Unix 秒。容器重启后继续使用原申请回执及发报记录。用真实 RF Link/FMO 客户端验证发现、登录和语音。

Dashboard 初始密码首次随机生成并存入持久卷。后台绑定 `127.0.0.1:18083`，用户名 `admin`。用下面的命令在自己的终端读取初始密码。

```sh
docker compose exec beacon /rf-link-server-beacon dashboard-password
```

远程访问 Dashboard 时，可用 SSH 隧道：

```sh
ssh -L 18083:127.0.0.1:18083 user@your-server
```

备份保留 `.env` 和全部命名卷。普通 `docker compose down` 保留数据。升级时使用对应版本的 Compose 和镜像，并复用同一项目的卷。编辑公告名称、SSID、网格或端点后重新执行 `docker compose up -d`。呼号绑定已申请身份，原申请人信息保留在本地。

仓库里可用 `docker compose -f compose.managed.yaml` 运行同一套配置。已有 FMO 服务继续按 [已有 FMO 教程](coexist.md) 添加 Cluster。此前的手工完整栈可继续使用仓库原有 `compose.yaml`，迁移时需先停止旧 Beacon，避免同身份重复发报。
