# RF Link Server Beacon

为你的 FMO / RF Link 服务器发布发现公告。填写呼号、SSID、网格坐标和服务器信息，程序自动申请广播身份、领取凭据，并每 **600 秒**发送一条经过验签的 STATION 公告。

广播私钥由远端 Trust 托管，本机只保存申请回执、受限调用凭据和公开证书。公告名称、地址和位置由你管理；修改本地配置并重启即可。支持 Docker 和 Linux systemd。

## 先选择目标

| 目标 | 使用教程 | SAS和MQTT怎么处理 |
|---|---|---|
| 新建RF Link服务器 | [新建教程](docs/operations/full-server.md) | 新部署MQTT + RF Link SAS |
| 保留FMO，同时增加RF Link | [兼容扩展](docs/operations/coexist.md) | 原FMO不变，新增独立SAS和MQTT监听端口，广播新端口 |
| 将FMO转换为RF Link | [转换教程](docs/operations/convert.md) | 备份后替换原SAS身份，可沿用原MQTT端口，不再保留旧FMO身份接入 |

先安装Git、Docker和Compose v2并获取源码：

```sh
git clone https://github.com/BI8SYN/rf-link-server-beacon.git
cd rf-link-server-beacon
```

三条流程都只有四步：**申请/等待 → 按提示配置 → 验证客户端 → 开始广播**。跟着所选教程填自己的呼号、SSID、网格和服务器资料即可；UID、指纹自动生成。

取得身份后运行 `./beacon.sh sas-command`，终端会先问你选哪种场景。脚本调用请明确指定：

```sh
./beacon.sh sas-command --scenario new-server
./beacon.sh sas-command --scenario coexist
./beacon.sh sas-command --scenario convert
```

只有选择convert才提供替换原身份的步骤。程序只生成配置，不自动修改Broker或SAS。`setup` / `wait`不广播；客户端验证完成后才执行 `./beacon.sh doctor` 和 `./beacon.sh start`。

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

支持 Linux systemd 原生部署，以及已有完整身份时的一行 Docker 部署，见[部署说明](docs/operations/deployment.md)。`deploy-docker` / `deploy-native` 会在身份就绪后自动广播，**不会自动配置 SAS**，新建服务器按完整服务器教程操作；兼容扩展和转换分别按对应教程操作。

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
