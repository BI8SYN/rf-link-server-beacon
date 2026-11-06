---
last-reviewed: 2026-11-06
---
# 开发交接

先读 README → project-brief → spec → architecture → contracts → protocol。源码包含 config、identity、protocol、signer、aprs、app 和 cli；模块名与公开 GitHub 路径一致。

已实现：自助申请、领取/续领凭据、链/CRL 验证、受限代签、CERT 重试快照、本地组包、Docker 单目录与 systemd 部署入口、SAS 配置代码（宿主机识别实际部署并按类型输出）、独立 EMQX/SAS 完整栈 Compose。

协议与实际广播已验证；完整栈验证范围单独记录在 [公开验收记录](development/open-source-validation.md)。不得将容器 healthy 等同合法设备认证、语音通信或官方设备发现兼容性通过。原生 systemd 仅做隔离检查，宿主机安装仍待测试。

## 后续验收

- 使用管理员自己的完整身份配置服务器，验证合法 FMO / RF Link 客户端登录、目标 UID/指纹和语音收发。
- 验证官方设备的实际信任根与发现行为。
- 测试真实 Linux systemd 首次安装、升级和卸载。
- 明确实际部署的备份、升级、恢复及告警策略；远端托管与审批可用性由 Trust 运营方保障。

## 开发检查

```sh
go test -race ./...
go vet ./...
go mod verify
sh -n beacon.sh
# 完整栈检查先配置.env；CI使用合成参数。
docker compose config --quiet
docker compose -f compose.beacon.yaml config --quiet
```

CI 使用临时密钥和回环服务，禁止生产申请、签名及 APRS 发报。个人运维资料留在忽略的 data/，不能发布到源码或镜像。不要操作生产 CA；实际生产发报或服务变更需明确授权。

2026-11-06流程调整：优先简单教程，无需全自动应用SAS。首次接入使用setup/wait、sas-command配置、客户端验证、doctor/start四步；README不再先推荐身份就绪后自动发报的deploy-docker。脚本setup/wait成功提示下一步，仍不发报。

## 三种业务场景

sas-command必须明确选择new-server、coexist或convert，再确定运行方式。新建默认完整栈；兼容扩展创建独立RF Link SAS与同Broker的新监听端口，原FMO身份/端口/全局认证保持；只有convert才给出备份后替换原SAS身份的步骤，可沿用旧端口且不再保留原FMO身份接入。

compose.sas.yaml只创建新的RF Link SAS，独立卷和HTTP端口；可用compose.sas-network.yaml加入现有Broker网络。EMQX 5.8.9监听器片段通过CLI生成，不自动应用生产配置。受控HTTP后端的双监听正向/错误身份/后端故障、双向消息、ACL和撤回测试通过，真实FMO/RF Link客户端仍须部署者验收。
