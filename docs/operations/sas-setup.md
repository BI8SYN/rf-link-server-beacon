---
last-reviewed: 2026-11-06
---
# 按场景生成SAS配置

先按[README](../../README.md)选择[新建](full-server.md)、[兼容扩展](coexist.md)或[转换](convert.md)，完成申请并等待签发，然后执行：

```sh
./beacon.sh sas-command
```

终端先询问业务场景。新建默认完整栈Compose，兼容扩展默认独立SAS Compose，转换才检测现有SAS的运行方式。不读取容器环境变量或私钥，不修改服务、不发报；输出前重新验链及CRL，不竞争广播进程的状态锁。

## 非交互脚本

```sh
./beacon.sh sas-command --scenario new-server --deployment full-stack
./beacon.sh sas-command --scenario coexist --mqtt-network 原Broker网络名
./beacon.sh sas-command --scenario convert --deployment native
```

业务场景不能省略。`--deployment compose|docker|native`选择SAS运行方式，不能决定是否覆盖原FMO身份；full-stack只允许new-server。兼容扩展即使指定docker，也使用独立Compose来管理新SAS，不重建原SAS容器。

兼容扩展的 `--fmo-port` 默认1883，新公告端口必须不同；修改本地 `server.port` 即可，不需重新申请。`--sas-http-port`默认18081，宿主机原生Broker通过该回环端口访问新SAS；Docker Broker指定 `--mqtt-network` 后通过同网络的 `http://rf-link-sas:8080/auth` 访问。高级拓扑可用 `--auth-url` 明确指定回调，必须先确保网络与监听受限且可达。新SAS数据与原服务隔离。

## 使用生成结果

新建完整栈输出六项SAS环境字段；兼容扩展输出新SAS环境字段/启动步骤及EMQX增量监听器片段；转换输出选定原服务的身份参数及备份提醒。不要把包含HOCON的兼容扩展整段输出写成.env或覆盖原Broker配置，按对应教程提取字段并增量合并。

UID、基础呼号、用户证书TBS指纹和issuerSn来自已验证身份，MQTT公开地址/端口来自本地配置；SSID不进入SAS基础呼号。证书指纹不是Root或Intermediate指纹。

手工原生SAS默认配置目录和系统服务的实际路径可能不同，转换时以原服务的--config参数为准。新建原生SAS使用独立的 `~/.rf-link-sas`；兼容模式默认独立HTTP端口，不修改原FMO服务。Docker完整/附加SAS模板均会安装已认可的两份公开根。

[官方SAS说明](https://github.com/BG5ESN/fmo-server-authrozier-service) · [官方建服教程](https://bg5esn.com/docs/fmo-server-construction/)
