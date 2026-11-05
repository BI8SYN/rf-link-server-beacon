---
last-reviewed: 2026-11-05
---
# 项目上下文

RF Link Server Beacon 让服务器管理员自助申请广播身份、配置 SAS，并发布 FMO V4 STATION 公告。Go 程序支持 Docker 与 systemd；已有 MQTT / SAS 可只部署广播，新建服务器使用独立 EMQX + SAS Compose。

基本资料为呼号、SSID、网格、服务器名称和 MQTT 地址，首次申请补充负责人姓名、邮箱及资格/用途说明。APRS passcode 自动计算。人工审核、CA 签发和发布后，程序领取受限调用凭据；广播私钥位于远端，CA 不参与在线公告签名。

公告配置由本地管理，Trust 仅负责身份和受限代签。每次请求完整 station 快照，验证身份、证书链、CRL、请求绑定、时间桶和 SIG 后，本地组装无空格 IFMO-V4 报文。待签快照保留原 CERT、链、哈希和正文，断线重试不改变请求 ID。

广播固定 600 秒，Docker 单 data 挂载，凭据提前续领，进程锁防止同目录重复运行。迁移可复制完整 data，但必须先停止来源。不能承诺宿主机 root 无法复制调用凭据；权限约束须由远端实施。

SAS 与广播使用同一公开身份，`sas-command` 生成部署代码，管理员应用后验证登录。完整栈首次初始化两个公开根，并在 Broker 启动前配置 SAS 认证。FAS 为独立可选审计服务，不属于本项目运行依赖。

源码公开仓库为 https://github.com/BI8SYN/rf-link-server-beacon，采用 MIT。详细能力和限制见[产品需求](spec/product-spec.md)与[交接](handoff.md)。
