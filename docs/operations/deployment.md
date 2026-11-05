---
last-reviewed: 2026-11-05
---
# 部署、备份与迁移

新建 MQTT / SAS 见[完整服务器教程](full-server.md)；只加广播见[README](../../README.md)。所有命令在源码目录执行，Docker Compose v2 为必需。首次构建需下载固定 Go 镜像，运行镜像不含凭据或源码。

## 广播服务

`deploy-docker` 一行初始化并后台运行；已有 `data/config.json` 时不重新初始化，不允许额外初始化参数。`setup` 只初始化并提交申请；`wait` 等待审核并领取；`start` 明确启用广播；`once` 请求签名但不发送，`once --send` 才发一条。`doctor` 校验证书链和 CRL，不发报。`sync-identity` 导出新 SAS 候选文件，不覆盖实际服务。`sas-command` 只读生成代码，可与广播守护进程同时运行。

Docker 数据默认 `./data`，可用 `BEACON_DATA_DIR=/绝对路径` 覆盖；容器仅挂载这一目录，无入站端口、Docker socket 或 privileged。使用 `unless-stopped`、只读根文件系统、cap_drop ALL 和日志轮换。主机应启用 Docker 开机启动。源码中的 `.env` 不属于 Beacon 必填配置。

简单配置见 [样例](../../configs/beacon.example.json)，高级 JSON 见[高级样例](../../configs/beacon.advanced.example.json)。国家默认 CN，范围 5000 公里，统计 0/0，间隔固定 600 秒。配置只在启动时读取，修改后重启。

## 原生 Linux systemd

需 Linux、systemd、root 和 Go 工具链。`sudo ./beacon.sh deploy-native ...` 创建 `rf-link-beacon` 账号，将程序安装到 `/usr/local/lib/rf-link-server-beacon/beacon`，资料放在 `/var/lib/rf-link-server-beacon`。进程退出失败后 15 秒重启。

```sh
sudo systemctl status rf-link-server-beacon
sudo journalctl -u rf-link-server-beacon -f
sudo systemctl stop rf-link-server-beacon
sudo systemctl start rf-link-server-beacon
```

命令行维护可显式使用该二进制与实际配置路径；运行中的服务持有状态锁，不要并行运行另一个发报进程。`sas-command` 是只读例外。macOS 开发可用 `BEACON_NATIVE=1 ./beacon.sh setup ...`，不提供 macOS 常驻安装器。原生单位文件做过隔离检查，宿主机安装尚未实测。

## 迁移与备份

1. 停止源广播实例，确认不会自动恢复或由另一守护进程拉起。
2. 安全备份并复制完整 `data/`，包括回执、机器凭据、配置、原待签 CERT/链、CRL 与广播防重复记录。目录 0700、文件 0600；不要放进 Git 或镜像。
3. 在目标机器获取同版源码，将数据放到 `./data`，执行 `./beacon.sh deploy-docker`，不加初始化参数。
4. 观察 `status` / `doctor` 和实际接收，再处理旧机器。不要恢复旧快照覆盖更新的防回滚记录，不要同时发报。

如果迁移完整服务器，还需 `.env` 和 SAS / EMQX 命名卷备份；仅迁 Beacon 不会迁移或修改原 SAS。用户可以复制调用凭据，本机加密不构成抵抗宿主机 root 的隔离。广播私钥始终位于远端签名设施，凭据只授权受限接口。

## 证书和根

RF Link 根内置在程序中；中间证书及用户证书从公开身份响应获得后验链。CRL 来源从已验证证书取得并受来源限制，过期/未知状态停止广播。未知根不能自动授信。

自建服务使用 `--service-url` 和经明确核验的 `--trust-root`，公开根被复制进 `data/trusted-root.json`。本机开发允许明确的回环 HTTP，不能把 localhost CRL 或测试根用于生产。SAS 根与 Beacon 根独立：容器路径通常是 `/home/app/.sas/roots`，原生以实际用户/启动参数为准。

## 升级

先备份并停止广播，`git pull --ff-only` 后运行 `./beacon.sh deploy-docker` 重建并复用原数据。不要重新 `setup`。SAS/EMQX 使用固定摘要，升级由管理员审阅上游兼容性后单独进行；仅更新 Beacon 不会改变它们。认证/证书轮换后，用 `sas-command` 核对新身份并应用到 SAS，再验证登录和广播。
