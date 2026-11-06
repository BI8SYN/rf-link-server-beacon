# RF Link Server Beacon

为你的 FMO / RF Link 服务器发布发现公告。填写呼号、SSID、网格坐标和服务器信息，程序自动申请广播身份、领取凭据，并每 **600 秒**发送一条经过验签的 STATION 公告。

广播私钥由远端 Trust 托管，本机只保存申请回执、受限调用凭据和公开证书。公告名称、地址和位置由你管理；修改本地配置并重启即可。支持 Docker 和 Linux systemd。

## 选择你的情况

| 当前情况 | 跟着哪份教程操作 |
|---|---|
| 已有能用的 MQTT / SAS 服务器 | 下方四步，给原服务器增加广播 |
| 还没有服务器，想从零搭建 | [完整服务器教程](docs/operations/full-server.md)，含 EMQX + SAS Compose |

两条流程都是：**申请身份 → 配置 SAS → 验证客户端 → 开始广播**。首次配置完成前使用 `setup`，最后再用 `start` 启用广播。

## 已有服务器：四步启用广播

需要 Git、Docker 和 Docker Compose v2。在服务器终端下载源码：

```sh
git clone https://github.com/BI8SYN/rf-link-server-beacon.git
cd rf-link-server-beacon
```

### 1. 申请身份并等待签发

替换以下呼号、网格、服务器地址和负责人资料。`YOURCALL` 必须换成自己的基础呼号，不包含 SSID。

```sh
./beacon.sh setup --callsign YOURCALL --ssid 12 --grid OL15ic --name "RF Link" --host mqtt.example.com --port 1883 --owner "负责人姓名" --email owner@example.com
./beacon.sh wait
```

申请需要 Trust 管理员审核、人工签发和发布。`wait` 会等待并自动领取凭据，显示 `ready: true` 后继续。这两条命令都不发广播；需要中断等待时按 Ctrl-C；之后回到本目录再次运行 `wait` 即可，不用重新申请。

### 2. 按提示配置原 SAS

```sh
./beacon.sh sas-command
```

程序识别实际 SAS 部署，只给对应的命令和操作说明；识别不到时选择你的部署方式。按输出更新原 SAS 并重启，保留原数据和信任根。UID、证书指纹等已自动填好，不用自己计算。此命令只生成配置，不会替你应用。

### 3. 验证服务器

用自己的合法 FMO / RF Link 客户端连接 `mqtt.example.com:1883`，确认能登录、收发语音，再检查广播身份：

```sh
./beacon.sh doctor
```

客户端登录或检查失败时，先按[SAS 配置说明](docs/operations/sas-setup.md)排查，成功后再继续。

### 4. 启动广播

```sh
./beacon.sh start
./beacon.sh logs
```

这一步才开始每 600 秒广播。容器故障或主机重启后自动恢复，只持久化一个 `data/` 目录。

## 日常管理

```sh
./beacon.sh status       # 申请及身份状态
./beacon.sh logs         # 广播日志
./beacon.sh doctor       # 检查身份、证书链及CRL，不发报
./beacon.sh stop         # 停止广播
./beacon.sh start        # 启用周期广播
```

基础配置在 `data/config.json`，首次申请所需负责人信息由脚本写入：

```json
{
  "callsign": "YOURCALL",
  "ssid": 12,
  "grid": "OL15ic",
  "server": {"name": "RF Link", "host": "mqtt.example.com", "port": 1883}
}
```

修改名称、SSID、网格或端点后，执行 `docker compose -f compose.beacon.yaml restart beacon`。保留已有 `applicant` 和其他配置，不要删除 `state/` 重新申请。SSID 范围 0–15；网格支持 4、6、8、10 位，位置取网格中心；APRS passcode 由基础呼号自动计算，无需填写。

当前广播周期固定为 600 秒，符合 [FMO STATION 协议建议](https://bg5esn.com/docs/fmo-aprs-formate/)，不提供任意缩短间隔的配置。统计暂为静态 0/0，不采集 Broker 在线人数。签名服务验证失败、证书失效或授权撤销时停止发送；恢复后不追补积压报文。

## 其他部署方式

支持 Linux systemd 原生部署，以及已有完整身份时的一行 Docker 部署，见[部署说明](docs/operations/deployment.md)。`deploy-docker` / `deploy-native` 会在身份就绪后自动广播，**不会自动配置 SAS**，首次接入推荐使用上方四步。

## 证书、凭据与迁移

默认 Trust 为 [trust.bi8syn.com](https://trust.bi8syn.com/)，RF Link 公开根随程序交付并固定核验；中间证书和用户证书由身份接口提供，再验链及 CRL。SAS 的根证书与 Beacon 分开配置，完整栈会安装[两份公开根](deploy/trust/README.md)。未知根不会因接口返回而自动受信。

迁移时先停止旧实例，再安全复制**完整 `data/`** 到新机器，执行 `./beacon.sh deploy-docker`，不再传初始化参数。回执、调用凭据、原待签 CERT 和防重复记录都要保留。凭据可以复制，必须保护备份，不能同时运行同一身份的多个副本。

## 文档与开发

[完整服务器](docs/operations/full-server.md) · [SAS 配置](docs/operations/sas-setup.md) · [部署/备份/升级](docs/operations/deployment.md) · [架构](docs/architecture/overview.md) · [签名接口](docs/contracts/remote-signer-v1.md) · [贡献](CONTRIBUTING.md)

```sh
go test -race ./...
go vet ./...
go mod verify
sh -n beacon.sh
# 完整栈需先按教程准备.env；独立广播Compose无需.env身份字段。
docker compose config --quiet
docker compose -f compose.beacon.yaml config --quiet
```

Go 构建基线 1.24，Docker 构建固定 Go 1.27.3 镜像摘要，依赖及许可证在 `vendor/`。CI 只用临时密钥和回环服务，不申请生产身份、不发送公网 APRS。完整栈、原生安装和设备兼容性验证范围见[验收记录](docs/development/open-source-validation.md)。

本项目代码采用 [MIT](LICENSE)；SAS、EMQX 和依赖各遵循其自身许可证，见[第三方说明](THIRD_PARTY_NOTICES.md)。
