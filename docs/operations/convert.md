---
last-reviewed: 2026-11-06
---
# 将FMO服务器转换为RF Link

此模式复用原MQTT和SAS，但**替换服务器身份，不再保留原FMO身份接入**。若要两边同时使用，请改走[兼容扩展教程](coexist.md)。

## 1. 先备份，再申请RF Link身份

备份原SAS的实际配置、管理员列表、信任根、Compose/系统服务及相关数据卷，记录原UID、指纹、端口和认证回调，安排维护窗口。按[README](../../README.md)下载源码，在仓库内申请：

```sh
./beacon.sh setup --callsign YOURCALL --ssid 12 --grid OL15ic --name "RF Link" --host mqtt.example.com --port 1883 --owner "负责人" --email owner@example.com
./beacon.sh wait
```

换成自己的资料和原MQTT公开端口；暂不广播。

## 2. 明确选择转换并应用

```sh
./beacon.sh sas-command --scenario convert
```

程序识别原SAS运行方式，只输出对应参数，提示转换的影响。识别不唯一时选择实际部署，脚本可用 `--deployment compose|docker|native` 指定。只有明确的convert场景提供替换原身份的步骤，不使用完整栈Compose新建另一个Broker。

将身份和MQTT字段合并到原SAS实际配置或启动参数；**保留其他管理员配置、卷和信任根**，必要时增加已核验的RF Link Root。系统服务使用的 `--config` 路径可能不同于手工运行的默认路径，应沿用实际路径。重启原SAS，不另开第二个同端口进程；确认原MQTT认证仍回调这个SAS，无需新开MQTT端口。

## 3. 验证后广播

用合法RF Link客户端连接原MQTT公开端口，确认身份、登录和语音正常；不要把旧FMO身份失败误当作需要关闭认证。

```sh
./beacon.sh doctor
./beacon.sh start
./beacon.sh logs
```

## 回滚

停止广播，恢复备份的原SAS配置、身份及启动方式，重启并验证原FMO接入。不要删除Beacon的完整data或恢复旧的CRL/发报状态来回滚服务器配置。
