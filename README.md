# RF Link Server Beacon

为你的 FMO / RF Link 服务器发布发现公告。填写呼号、SSID、网格坐标和服务器信息，程序自动申请广播身份、领取凭据，并每 **600 秒**发送一条经过验签的 STATION 公告。

广播私钥由远端 Trust 托管，本机只保存申请回执、受限调用凭据和公开证书。公告名称、地址和位置由你管理；修改本地配置并重启即可。支持 Docker 和 Linux systemd。

- **已有 MQTT / SAS**：按下方快速部署增加广播，用 `sas-command` 配置原 SAS。
- **从零建服务器**：按[完整服务器教程](docs/operations/full-server.md)启动本仓库的 EMQX + SAS Compose，验证登录后启用广播。

## 快速部署：已有服务器

先安装 Git、Docker 和 Docker Compose v2，然后获取源码：

```sh
git clone https://github.com/BI8SYN/rf-link-server-beacon.git
cd rf-link-server-beacon
```

将以下示例换成你自己的呼号、网格和服务器资料，一行部署并启用后台广播：

```sh
./beacon.sh deploy-docker --callsign YOURCALL --ssid 12 --grid OL15ic --name "RF Link" --host mqtt.example.com --port 1883 --owner "负责人姓名" --email owner@example.com
```

`YOURCALL` 是占位符，必须替换。申请需要人工审核、签发和发布，容器会自动等候；身份就绪后自动领取凭据并开始广播。请先确保目标 MQTT / SAS 已就绪。只准备申请、暂不广播时使用 `setup` 替换 `deploy-docker`，发布后运行 `wait`。

取得身份后生成 SAS 配置代码，按原生或容器方式应用到你的 SAS：

```sh
./beacon.sh sas-command
```

命令自动填入本人 UID、用户证书指纹、签发者、基础呼号及 MQTT 地址；不会修改或启动 SAS。参阅[SAS 配置](docs/operations/sas-setup.md)，配置完成后验证合法客户端登录。

容器故障或主机重启后自动恢复，数据仅持久化到一个 `data/` 目录。安装使用源码构建，不需要镜像仓库账号，也不下载并执行远程脚本。

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

## 直接安装到 Linux

有 systemd 和 Go 工具链的 Debian / Ubuntu 类 Linux，可在源码目录执行：

```sh
sudo ./beacon.sh deploy-native --callsign YOURCALL --ssid 12 --grid OL15ic --name "RF Link" --host mqtt.example.com --port 1883 --owner "负责人姓名" --email owner@example.com
```

创建独立系统账号，数据放在 `/var/lib/rf-link-server-beacon`，故障 15 秒后重启，开机自动运行。用 `journalctl -u rf-link-server-beacon -f` 查看日志。macOS 可使用 `BEACON_NATIVE=1` 运行前台命令，不安装 systemd。更多操作见[部署与迁移](docs/operations/deployment.md)。

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
