---
last-reviewed: 2026-11-06
---
# 系统架构

```text
管理员 ──申请/授权──> 独立签名与身份服务（不在用户 Compose 中）
                         │ 用户身份密钥 / HSM
用户 Docker 主机         │
  Beacon ──HTTPS结构化请求─┘
    │ ← 公开证书与签名；本地复验
    └──APRS-IS STATION──> 其他终端（独立判定 Root 信任）

  MQTT Broker ──HTTP认证──> SAS（官方与 RF Link Root）
       └──规则/webhook/API──> FAS（可选）
```

## 初始化协调

完整栈按教程分阶段初始化：setup申请 → 人工签发 → wait领取并验链 → sas-command生成身份字段 → 管理员写入.env → sas-init初始化公开根 → SAS healthy → EMQX启动。合法客户端验收后由管理员start启用广播。

sas-init只复制已审核的公开根并在首次创建配置时关闭SAS自动更新，不持有机器凭据或Docker socket。SAS通过显式CLI参数配置同一身份，持久卷独立于Beacon的data。EMQX从第一次启动即配置HTTP认证，SAS失败时拒绝连接；不依靠临时匿名开放安装。

仅Beacon模式不写外部SAS，sas-command由宿主机脚本只读识别官方SAS容器元数据或原生SAS安装，显式选择后传给Go程序；仅生成对应full-stack/compose/docker/native代码。识别不到或不唯一时，终端选择，非交互明确失败。不把Docker访问能力交给容器。现有管理员必须备份、合并和应用配置，不自动替换已有SAS。

## 模块

config：严格配置；identity：公开链验证；protocol：确定性CBOR和STATION；signer：远程契约与绑定；aprs：有界行解析、验证登录与重连；app：调度与生命周期。各模块测试使用临时密钥和回环服务，集成验收范围见development/open-source-validation.md。

Trust只托管身份和受限STATION签名；Beacon持有运行配置并持久保存完整签名请求快照。响应验签以快照为准，不从身份查询取得station/profileVersion。旧申请快照仅为幂等恢复保留，旧profile待签请求在切换启动时作废，身份和凭据不重建。

## 远端边界

私钥不进入管理员主机、镜像、环境变量、卷或备份。远端令牌仍可被管理员复制，需用节点绑定、参数白名单、短期令牌、撤销与审计控制。签名服务验证“签名权限”与证书 CA 验证不是一回事。

CRL 静态分发、证书审批、广播签名可分服务。若接入现有 fmo-trust，必须保留既有人工作业/1Password 边界，不可把其 CA 签名路径暴露为网络接口。
