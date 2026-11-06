---
last-reviewed: 2026-11-06
---
# 保留FMO，同时增加RF Link

保留原FMO的SAS、UID/指纹、MQTT端口和认证配置。在**同一Broker**新增RF Link端口，由一个新SAS认证，广播这个新端口。示例原端口1883、新端口1884。

下面的监听器模板已在 **EMQX 5.8.9** 验证。其他版本或Broker不要直接套用；先确认支持独立监听器认证。不创建第二个Broker，也不修改全局authentication/authorization。

## 1. 申请新入口的身份

安装Git、Docker及Compose v2，下载本仓库，在仓库目录执行：

```sh
./beacon.sh setup --callsign YOURCALL --ssid 12 --grid OL15ic --name "RF Link" --host mqtt.example.com --port 1884 --owner "负责人" --email owner@example.com
./beacon.sh wait
```

替换呼号、网格及服务器资料。等待审核签发，看到ready后继续。这时不广播。已有申请不必重申请，只需将 `data/config.json` 的 `server.port` 改为新端口。

## 2. 生成新SAS和新监听器配置

如果原MQTT在宿主机原生运行：

```sh
./beacon.sh sas-command --scenario coexist > data/rflink-setup.txt
```

如果原MQTT在Docker，先查看它的网络名（替换原MQTT容器名），再生成：

```sh
docker inspect --format '{{json .NetworkSettings.Networks}}' 原MQTT容器名
./beacon.sh sas-command --scenario coexist --mqtt-network 原MQTT网络名 > data/rflink-setup.txt
```

查看 `data/rflink-setup.txt`。仅首次部署时提取新的SAS配置，**不覆盖原FMO的.env**：

```sh
umask 077
sed -n '/^SAS_/p;/^RF_LINK_/p' data/rflink-setup.txt > .env.rflink
```

按生成结果中的启动命令运行专用 `compose.sas.yaml`；MQTT在Docker时该命令会同时使用 `compose.sas-network.yaml`。它只启动RF Link SAS，拥有独立卷和项目名，不启动MQTT或修改原SAS。原生SAS方案可显式加 `--deployment native`，按输出新建独立配置与守护服务，不修改原FMO服务。

## 3. 给原MQTT增加一个监听入口

先备份原Broker的实际生效配置、Compose及原SAS配置。将生成结果的 `listeners.tcp.rf_link { ... }` **增量合并**到原EMQX实际生效的HOCON配置，保留全部其他内容。不要把整份结果当作配置覆盖，也不要将认证器加入全局数组。若已有同名监听器，先核对其用途，不能盲目覆盖。

Docker Broker还需在原Compose的MQTT服务中**追加** `"1884:1884"` 端口映射，保留原映射、镜像、卷及网络。放行新端口防火墙；SAS HTTP端口不要公开到互联网。配置应用方式以现有部署为准，安排好可能的Broker重建/重启窗口。

新SAS、MQTT监听器、端口映射和公告的公开端口均应对应1884。原FMO端口不是1883时，生成命令加 `--fmo-port 实际旧端口`；新SAS本机HTTP端口18081被占用时加 `--sas-http-port 其他空闲端口`。不要靠端口映射复用旧监听器，它仍会使用旧认证。

## 4. 两边都验收，再广播

确认：原FMO客户端仍从旧端口登录、收发语音；RF Link客户端从新端口登录、收发语音；同房间跨端口互通与原ACL行为正确；已有FAS时也要验证它不会误封新入口。检查错误身份/匿名登录应被拒绝。Docker healthy不代表真实客户端已通过。

```sh
./beacon.sh doctor
./beacon.sh start
./beacon.sh logs
```

从这一步开始广播新端口。未通过实际客户端验收前不要start。

## 撤回与备份

先 `./beacon.sh stop`，移除新增的rf_link监听器/端口映射并按原部署应用，然后用原来的专用Compose命令 `down` 停止RF Link SAS。不要删原FMO卷或执行原项目的 `down -v`。保留 `.env.rflink`、SAS卷和完整 `data/`；独立SAS停止后，旧FMO服务应仍能认证。

模板只生成配置，不自动连接管理API或修改原Broker。若同机容器网络、代理或TLS不同于上述两种拓扑，先调整新SAS受限监听与网络，再用 `--auth-url http(s)://实际可达地址/auth` 指定Broker回调；跨主机配置须自行验收。
