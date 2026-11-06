---
last-reviewed: 2026-11-06
---
# 从零搭建 FMO / RF Link 服务器

跟着四步完成：**申请身份 → 启动 SAS / MQTT → 验证客户端 → 广播**。只需填写呼号、SSID、网格、服务器资料和负责人信息，SAS 的 UID、指纹等由程序生成。

已有FMO服务器请先看[兼容扩展设计](../plans/2026-11-06-fmo-coexistence.md)。它需要同一Broker新增端口及独立RF Link SAS，不能用本教程替换原服务器。见[保留FMO增加RF Link教程](coexist.md)；若要放弃原FMO身份，见[转换教程](convert.md)。

## 准备

服务器安装 Git、Docker、Docker Compose v2。准备客户端能访问的域名或公网地址，将域名解析到本机，先在云安全组或覆盖Docker流量的防火墙中仅允许验收客户端访问TCP 1883，验证后再按运营策略放行。本教程使用普通 MQTT 1883。

```sh
git clone https://github.com/BI8SYN/rf-link-server-beacon.git
cd rf-link-server-beacon
```

后续命令都在这个目录执行。将 `YOURCALL`、网格和 `mqtt.example.com` 换成自己的资料。

## 1. 申请身份并等待签发

```sh
./beacon.sh setup --callsign YOURCALL --ssid 12 --grid OL15ic --name "RF Link" --host mqtt.example.com --port 1883 --owner "负责人姓名" --email owner@example.com
./beacon.sh wait
```

申请提交给 Trust，由管理员审核、人工签发和发布。看到 `ready: true` 后继续；这时还没有广播。需要中断等待时按 Ctrl-C；之后再次运行 `wait` 即可，不用重新申请。

## 2. 生成配置并启动服务器

以下是**首次部署**命令，会生成 `.env`，不需要手抄 UID 或指纹；已有 `.env` 时不要覆盖，改用 `sas-command --deployment full-stack` 显示结果后合并 SAS 字段。

```sh
umask 077
./beacon.sh sas-command --scenario new-server --deployment full-stack > .env
```

确认命令成功、`.env` 中包含六项 `SAS_` 配置后，用编辑器打开它，在末尾添加一行自己的随机长密码：

```dotenv
EMQX_DASHBOARD_PASSWORD=替换为你自己的随机长密码
```

这是 MQTT 管理后台密码，默认用户名 `admin`。只保存在本机 `.env`，不要提交到 Git。如果生成失败，修复提示的问题后重新运行生成命令，再继续。

启动 SAS 与 MQTT：

```sh
docker compose up -d --wait
```

成功后两个服务应显示 healthy。本模板会安装官方与 RF Link 两份公开根、配置 SAS 身份和 MQTT 认证，不需要手工操作证书目录。**这里仍未启用广播。**

## 3. 验证客户端

用自己的合法 FMO / RF Link 客户端连接 `mqtt.example.com:1883`，确认能登录、收发语音。再检查广播身份：

```sh
./beacon.sh doctor
```

容器healthy或端口已监听都不代表HTTP认证已就绪。EMQX 5.8.9冷启动存在先监听、后安装认证hook的上游窗口；quick_deny_anonymous只能提前拒绝无用户名连接。维护期间限制外部接入，验证合法登录与错误/匿名登录拒绝后再正式放行。每次Broker冷启动均须考虑该限制，详见[启动边界](coexist.md#emqx-589冷启动边界)。本项目没有实现外部启动流量门禁。

## 4. 开始广播

```sh
./beacon.sh start
./beacon.sh logs
```

此后每 600 秒广播，容器故障或主机重启后自动恢复。目标客户端是否能发现服务器，需要在客户端实际观察。

## 常用操作与排错

| 目的 | 命令 |
|---|---|
| 查看申请/身份 | `./beacon.sh status` |
| 查看 SAS / MQTT 状态 | `docker compose ps` |
| 排查认证或启动失败 | `docker compose logs --tail 50 sas emqx` |
| 查看广播日志 | `./beacon.sh logs` |
| 停止广播 | `./beacon.sh stop` |
| 停止 SAS / MQTT | `docker compose stop` |

SAS 8080 不对公网开放，管理后台只绑定本机 `http://127.0.0.1:18083`。需要远程管理时，在自己的电脑建立隧道后用浏览器打开该地址：

```sh
ssh -L 18083:127.0.0.1:18083 user@your-server
```

服务器已占用 1883 时不要重复部署另一套 Broker，不要用本教程覆盖原服务，先看[兼容扩展设计](../plans/2026-11-06-fmo-coexistence.md)。原生安装、混合网络见[SAS 配置](sas-setup.md)；TLS、FAS 审计等扩展见下方官方资料。

备份应包含 `.env`、完整 `data/` 及 SAS / EMQX 两个命名卷。`docker compose down` 保留命名卷，**不要用 `down -v` 作为停止命令，它会删除服务器数据**。升级与迁移见[运维说明](deployment.md)。

## 使用官方教程建服

[FMO 官方建服教程](https://bg5esn.com/docs/fmo-server-construction/)可用于了解官方安装。已经按它运行的FMO服务器，要兼容RF Link时必须新增独立SAS和MQTT端口，保留原FMO身份和认证。如果明确放弃旧FMO身份，可按[转换教程](convert.md)备份后更换原SAS身份。

[SAS 官方仓库](https://github.com/BG5ESN/fmo-server-authrozier-service) · [可选 FAS 审计教程](https://bg5esn.com/docs/fmo-fas-install-guide/)
