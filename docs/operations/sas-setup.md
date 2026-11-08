---
last-reviewed: 2026-11-07
---
# 按场景生成 SAS 配置

只保留两种场景：[完全新建 RF Link](full-server.md)和[已有 FMO 信任并增加 RF Link 广播](coexist.md)。申请发布、`wait` 领取身份后执行：

```sh
./beacon.sh sas-command
```

终端选择场景后，新建默认完整栈 Compose；已有 FMO 识别原 SAS 运行方式。无法唯一识别时选择，非交互脚本明确指定：

```sh
./beacon.sh sas-command --scenario new-server --deployment full-stack
./beacon.sh sas-command --scenario coexist --deployment native
```

`--deployment compose|docker|native` 选择实际 SAS 部署类型；`full-stack` 仅用于新建。新建 Docker 统一使用完整栈，原生输出独立配置及启动参数。已有 FMO 输出同一 SAS 的 Cluster 候选生成代码及对应部署的应用步骤，共用现有 MQTT 地址和端口。

程序输出前重新验链及 CRL，不修改服务、不发报、不续领凭据，不竞争广播进程的状态锁。UID、基础呼号、用户证书 TBS 指纹和中间 CA 序列号来自已验证身份，MQTT 地址/端口来自本地配置；SSID 不进入 SAS 基础呼号。证书指纹不是 Root 或 Intermediate 指纹。

兼容场景的 `tools/sas-config.py` 只离线读取原配置并生成新的 0600 候选，保留原 Server、管理员、Trust、HTTP、其他 Cluster 及未知字段。管理员安装认可根、按需扩展非空签发者允许列表，检查差异后写回并仅重启原 SAS。完整步骤见[已有 FMO 教程](coexist.md)。所有身份共享信任和权限策略；不能把 Cluster 当成独立安全域。

旧的 `convert`、`--fmo-port`、`--sas-http-port`、`--mqtt-network`、`--auth-url` 已移除，明确报错；不再生成替换原身份、第二套 SAS 或新监听器。已有旧方案实例不会被自动停止或修改。

[官方 SAS 说明](https://github.com/BG5ESN/fmo-server-authrozier-service) · [官方建服教程](https://bg5esn.com/docs/fmo-server-construction/)
