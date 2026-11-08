---
last-reviewed: 2026-11-07
---
# 产品需求

## 身份与证书

管理员填写呼号及负责人身份、资格和用途资料，完成远端审核、人工签发和发布。新申请不提交 station。远程生成广播用户密钥，仅返回公开链和身份；本机保存申请回执和受限凭据。未批准不能广播，相同请求重试不重复创建身份，凭据不进入 URL、日志、镜像或命令行。

## 广播

遵循 FMO V4 STATION 十六项 CBOR 与 600 秒时间桶；SSID 0–15，周期固定 600 秒。公告由本地配置，修改后重启，无需重新签证书。每次请求完整 station/requestId/online/peak，本地独立验签、验链与 CRL，构造 IFMO-V4 无空格报文，限制 480 字节。

持久快照包含原 CERT/链/两种哈希、身份与完整请求。断线重试保持原 ID 和正文，不混用轮换后的证书；吊销、过期、未知根、错误 SIG 或错配不得发送。进程恢复不追补积压，只发送当前有效公告。统计首版固定 0/0。

## 部署场景

### 新建RF Link服务器

使用独立EMQX + RF Link SAS + Beacon。RF Link公开身份用于该SAS，公告端口与它认证的MQTT端口一致。支持Docker及Linux原生运行；完整栈Compose为新建服务器入口。

### 已有FMO：信任并添加RF Link广播

保留原SAS主UID、指纹、管理员、根、数据以及原MQTT地址、端口和认证回调。在同一SAS的Mqtt.Clusters追加RF Link目标身份，并加入明确认可的RF Link根；不新建SAS或MQTT监听器。非空AllowIssuerSn须明确允许新签发者，不能清空旧限制。所有身份共享根、issuer过滤及管理员/角色策略，须检查跨CA的UID冲突及权限。

生成工具只写新的候选文件，不覆盖原配置，不修改服务或自动迁移旧双SAS实例。保留未知字段、其他Cluster和全部主身份；同身份重复运行幂等，冲突明确失败。证书轮换仅修改对应Cluster的指纹。

### 操作流程与验收

仅选择new-server或coexist，再识别SAS运行方式（Compose/Docker/原生）。首次流程为申请/等待 → 安装信任并配置身份 → 验证客户端 → doctor/start。只填写必要资料；UID、指纹等从已验证身份生成。转换及双SAS入口移除。

已有FMO验收覆盖：原FMO和RF Link通过相同端口合法登录、房间消息和语音；错误身份/匿名拒绝；现有ACL/FAS与权限正确；撤回新Cluster后原FMO继续使用。容器healthy或SAS配置加载不能代替合法客户端验收。

## 运维与非目标

提供 setup/wait/status/doctor/sas-command/once/start/stop/logs；once 默认不发 APRS。状态区分实际发送与对端显示，容器启动不等于合法客户端验收。项目不实现 Broker、SAS 认证算法、FAS、CA 签发或本地私钥模式，不承诺官方设备默认信任新增根。
