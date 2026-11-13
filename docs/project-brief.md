---
last-reviewed: 2026-11-07
---
# 项目上下文

RF Link Server Beacon 让服务器管理员自助申请广播身份、配置 SAS，并发布 FMO V4 STATION 公告。Go 程序支持 Docker 与 systemd；已有 MQTT / SAS 可只部署广播，新建服务器使用独立 EMQX + SAS Compose。

基本资料为呼号、SSID、网格、服务器名称和 MQTT 地址，首次申请补充负责人姓名、邮箱及资格/用途说明。APRS passcode 自动计算。人工审核、CA 签发和发布后，程序领取受限调用凭据；广播私钥位于远端，CA 不参与在线公告签名。

公告配置由本地管理，Trust 仅负责身份和受限代签。每次请求完整 station 快照，验证身份、证书链、CRL、请求绑定、时间桶和 SIG 后，本地组装无空格 IFMO-V4 报文。待签快照保留原 CERT、链、哈希和正文，断线重试不改变请求 ID。

广播固定 600 秒，Docker 单 data 挂载，凭据提前续领，进程锁防止同目录重复运行。迁移可复制完整 data，但必须先停止来源。不能承诺宿主机 root 无法复制调用凭据；权限约束须由远端实施。

SAS 与广播使用同一公开身份，`sas-command` 在宿主机识别实际SAS部署，只输出该方式的代码；未知或冲突时选择，脚本可用--deployment指定。管理员应用后验证登录。完整栈首次初始化两个公开根，并在 Broker 启动前配置 SAS 认证。FAS 为独立可选审计服务，不属于本项目运行依赖。

源码公开仓库为 https://github.com/BI8SYN/rf-link-server-beacon，采用 MIT。详细能力和限制见[产品需求](spec/product-spec.md)与[交接](handoff.md)。

面向管理员的使用流程不要求完全自动化。README和建服教程按完全新建、已有FMO两条主线；首次使用先setup/wait，再应用SAS并验证客户端，最后doctor/start。自动启动的deploy入口不作为首次接入的默认推荐。

## 两种部署场景

仅保留new-server完全新建和coexist已有FMO。新建Docker使用完整EMQX/SAS栈；已有FMO保留原主身份和管理员，在原SAS的Mqtt.Clusters追加RF Link目标身份，共用原MQTT地址和端口，根及认证策略共享。

sas-command在兼容模式识别原SAS部署，仅输出对应类型的配置步骤。tools/sas-config.py离线生成候选，只追加一个Cluster，保留原配置其他字段；拒绝UID/指纹冲突、端点不匹配、歧义键和已有输出文件，不自动应用。管理员备份、安装认可根、按需扩展非空AllowIssuerSn、检查差异后仅重启原SAS。证书轮换只更新对应Cluster指纹，不能用独立SAS导出配置覆盖原FMO配置。

转换入口、双SAS Compose和新监听器生成已移除，旧实例不自动迁移或删除。隔离容器配置加载测试与真实客户端/语音验收是不同范围，后者须部署者自行验证。

2026-11-09：deploy-native改为自动下载GitHub Release的Linux amd64/arm64二进制并验证SHA256SUMS，无需Go。CI交叉构建artifact，v* tag发布Release；BEACON_VERSION可固定版本。源码开发BEACON_NATIVE=1 setup仍可本地编译。
