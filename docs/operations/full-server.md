---
last-reviewed: 2026-11-05
---
# 从零搭建 FMO / RF Link 服务器

本教程使用 Docker Compose 部署 EMQX（MQTT）和官方 SAS（客户端身份认证），再用 Beacon 广播。已有服务器直接看 [SAS 配置代码](sas-setup.md)，无需重建。

服务器需有 Docker、Compose v2、Git，客户端能访问的域名或公网地址，开放 TCP 1883。域名解析到该服务器；云安全组和防火墙也需放行。Dashboard 只绑定本机，SAS 8080 只在 Docker 网络内使用。当前模板提供普通 MQTT 1883，TLS、反向代理和跨主机部署按实际环境配置。

## 1. 下载并申请广播身份

```sh
git clone https://github.com/BI8SYN/rf-link-server-beacon.git
cd rf-link-server-beacon
./beacon.sh setup --callsign YOURCALL --ssid 12 --grid OL15ic --name "RF Link" --host mqtt.example.com --port 1883 --owner "负责人姓名" --email owner@example.com
./beacon.sh wait
```

替换示例资料。`setup` 不广播，`wait` 等待 Trust 管理员审核、人工签证书和发布，然后自动领取受限调用凭据。审批入口及补充资料以申请状态和服务运营方要求为准；程序不能绕过人工审批。

## 2. 用广播身份配置 SAS

生成代码并提取六项 SAS 配置，不需要手填 UID、证书指纹或签发者：

```sh
umask 077
./beacon.sh sas-command > data/sas-commands.txt
# 首次部署使用；已有.env请合并SAS_行，保留自己的其他配置。
test ! -e .env && sed -n '/^SAS_/p' data/sas-commands.txt > .env
```

使用编辑器打开 `.env`，追加：

```dotenv
EMQX_DASHBOARD_USERNAME=admin
EMQX_DASHBOARD_PASSWORD=替换为随机生成的长密码
```

Dashboard 密码不需要传给 Beacon，也不放进 Git。可以用 `openssl rand -hex 24` 在本机生成随机值。不要原样使用上述密码说明。完整字段见 [`.env.example`](../../.env.example)。`SAS_MQTT_HOST` 应为客户端使用的公开域名；模板的监听端口与 `SAS_MQTT_PORT` 一致。

本项目 `compose.yaml` 已安装官方 Root 与 RF Link Root。`sas-init` 只将已审核的公开根复制到 SAS 持久卷，保留其他根；初次创建配置时关闭 SAS 自更新。SAS 使用上一步同一广播身份。Beacon 自身不需要持有任何私钥。

## 3. 启动 MQTT 和 SAS

```sh
docker compose config --quiet
docker compose up -d --wait
docker compose ps
docker compose logs --tail 50 sas emqx
```

Compose 依次完成公开根初始化、SAS 监听健康检查，再启动已启用 HTTP 认证的 EMQX；缺少身份或 Dashboard 密码时会直接报错。SAS 不可达或拒绝认证时，客户端不能匿名进入 Broker。不要为了排错关闭认证。

模板独立使用 `rf-link-server` 项目及 `sas-data`、`emqx-data` 命名卷，Beacon 仍使用单独的 `compose.beacon.yaml` 和 `data/`。默认运行的是 MQTT + SAS，FAS 可按[官方 FAS 教程](https://bg5esn.com/docs/fmo-fas-install-guide/)另行添加。

Dashboard 为 `http://127.0.0.1:18083`，远程访问使用 SSH 隧道：

```sh
ssh -L 18083:127.0.0.1:18083 user@your-server
```

在本机浏览器访问该地址。首次密码从 `.env` 设置；已有 EMQX 数据卷中的 Dashboard 密码不因修改 `.env` 自动重置。

## 4. 验证客户端后开始广播

先使用实际合法 FMO / RF Link 客户端连接你的公开 MQTT 地址，确认 SAS 认证及语音收发；再检查匿名与错误身份应被拒绝。容器 healthy 只表示进程状态，不代替实际客户端验收。

```sh
./beacon.sh doctor
./beacon.sh start
./beacon.sh logs
```

此后每 600 秒广播，Docker 与主机重启后自动恢复。实际出现在客户端服务器列表，还取决于客户端信任根及协议支持，需要在目标客户端观察。不要把 TCP 写入成功当作所有设备均已收到。

## 更新、备份和参考

`docker compose stop` 停止 MQTT / SAS；`./beacon.sh stop` 单独停止广播。`docker compose down` 保留命名卷，**`down -v` 会删除服务器数据**。同时备份 `.env`、完整 `data/` 和两个命名卷；更新镜像前保留旧版本及卷快照。不要在同一机器重复启动另一个占用 1883 的 Broker。

SAS 的拓扑、原生安装与网络说明以 [FMO 官方建服教程](https://bg5esn.com/docs/fmo-server-construction/)及 [SAS 官方仓库](https://github.com/BG5ESN/fmo-server-authrozier-service)为参考。按官方教程安装后，最后运行本项目 `./beacon.sh sas-command`，将生成的原生命令或 Compose 参数应用到原 SAS；保留原服务的数据、管理员配置和信任根。详细区别见 [SAS 配置](sas-setup.md)。
