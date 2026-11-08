---
last-reviewed: 2026-11-07
---
# 已有 FMO：信任并添加 RF Link 广播

继续使用原来的 MQTT 和 SAS，在原 SAS 的 `Mqtt.Clusters` 中追加 RF Link 服务器身份，广播原来的公开地址和端口。原 FMO 主身份、管理员和客户端入口保留。

需要支持 Cluster 目标身份的 SAS；本项目使用官方 1.0.6 验证配置加载。如果旧版不支持，先单独安排升级并验证原 FMO；不要覆盖原主身份。两个身份共用根、签发者限制和认证策略，无法分别设置隔离的信任域。

## 1. 申请身份

安装 Git、Docker、Compose v2 和 Python 3，下载本仓库。在仓库目录执行，填写**现有服务器的公开地址和端口**，选择未使用的 SSID：

```sh
./beacon.sh setup --callsign YOURCALL --ssid 12 --grid OL15ic --name "RF Link" --host mqtt.example.com --port 1883 --owner "负责人" --email owner@example.com
./beacon.sh wait
```

等待审核、人工签发和发布，看到 `ready: true` 后继续；这时不广播。已有身份保留完整 `data/`，不用重新申请。

## 2. 给原 SAS 增加信任和身份

```sh
./beacon.sh sas-command --scenario coexist
```

程序识别原 SAS 是 Compose、独立 Docker 还是原生服务，仅显示该方式的操作代码。无法唯一识别时选择，脚本可用 `--deployment compose|docker|native` 明确指定。没有 Docker 访问权限时在宿主机以有权限的账号执行，或明确运行方式。

按输出完成下面三件事：

1. 找到原进程**实际生效**的配置和 `Trust.RootsDir`，备份配置、根目录及启动配置。原生看 systemd 的 `ExecStart` / `--config` 及启动日志；Docker 看原 Compose、卷映射和容器启动日志，路径是容器内的。不要用当前登录账号的默认 `~/.sas` 猜测服务路径。
2. 把仓库的 `deploy/trust/roots/rf-link-root-production-v1.json` 加入实际根目录，保留原 FMO 根。确认是下表的认可根；已有同名文件时先比较，不盲目覆盖。若 `Trust.AllowIssuerSn` 非空，还须明确允许输出中的 RF Link 中间 CA 序列号，保留已有条目；空数组表示不限制签发者。不要为接入而清空原限制。
3. 运行输出中的 `python3 tools/sas-config.py ...`，生成 `data/sas-config.rflink.json`。工具只追加 Cluster，不更改原文件；核对差异后按输出注释写回原配置，并**仅重启原 SAS**。Docker 配置和根必须位于原持久卷，保留文件属主/权限；若为只读挂载，在宿主机修改对应源文件再重启。Broker 端口、认证回调和网络保持原样。

| 认可的 RF Link Root | 值 |
|---|---|
| TBS 指纹 | `lPu5cTZM9fOEgCw7nM8GjFPsdV7z8TXEU1QWHOytzow` |
| 文件 SHA-256 | `3b253cd8124342104f997b817574ff071d8f40c9a7638a012632296622722917` |

复制根的命令，根据原 SAS 所在位置执行一种（先替换实际路径/容器名）：

```sh
# 原生：SAS_ROOTS为运行中的SAS实际Trust.RootsDir
sudo cp deploy/trust/roots/rf-link-root-production-v1.json "$SAS_ROOTS/"
```

```sh
# Docker：SAS_ROOTS为容器内实际Trust.RootsDir，且位于持久卷
# Docker Compose也可使用docker cp，不需要进入容器找shell。
docker cp deploy/trust/roots/rf-link-root-production-v1.json "$SAS_CONTAINER:$SAS_ROOTS/rf-link-root-production-v1.json"
```

候选包含准确的五个 Cluster 字段：`uid`、`callsign`、`mqtt_host`、`mqtt_port`、`certFingerprint`。UID 和指纹来自已验证证书，不能填原 FMO UID。工具会拒绝不同端点、UID/指纹冲突、歧义 JSON 和已有输出文件；相同条目不重复添加。已有候选时保留备份、换一个输出文件名即可。原配置需要root权限读取时，为候选工具及diff命令加sudo；不要更改原配置的访问权限。

如果此前按旧教程用了新端口或双 SAS，先单独备份和确认原主身份，修改本地 `server.host/port` 为原入口，按本流程迁入；客户端验收后再人工撤下旧附加服务。新版不会自动删除旧服务、卷或端口。

## 3. 验证，再广播

查看原 SAS 重启日志，确认原 FMO 主 UID/指纹仍在，并加载新 RF Link Cluster 和两个认可根。用合法 FMO、RF Link 客户端分别通过**同一地址和端口**登录并验证语音和房间消息；检查错误身份、匿名连接被拒绝，原 ACL/FAS 行为符合预期。共享信任可能导致跨 CA 的用户 UID 与管理员策略相互影响，须核验实际权限，不能仅凭 healthy 判定成功。

```sh
./beacon.sh doctor
./beacon.sh start
./beacon.sh logs
```

从 `start` 开始，每 600 秒广播一次。`doctor` 只检查广播身份/链/CRL，不代替 SAS 客户端验收。

## 备份、轮换与撤回

保存原 SAS 配置与根目录、候选配置、启动配置和完整 Beacon `data/`。迁移先停止源实例再复制，不能同时运行同一身份的两个广播副本。

证书轮换后，只更新对应 RF Link Cluster 的 `certFingerprint`，核验后重启 SAS；不能把程序导出的独立 `sas-config-*.json` 整份替换原 FMO 配置。工具遇到同 UID 不同指纹会拒绝，要求人工检查轮换。

撤回时先 `./beacon.sh stop`，只移除新增 RF Link Cluster 并重启原 SAS。保留原主身份、其他节点和 Beacon 回执。是否继续信任 RF Link 根由管理员决定；只有确认无其他身份使用时才移除，不能还原过时整份配置覆盖后来改动。

[官方 SAS Cluster 管理](https://github.com/BG5ESN/fmo-server-authrozier-service#集群管理) · [SAS 配置说明](sas-setup.md)
