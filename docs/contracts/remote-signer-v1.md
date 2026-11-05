---
last-reviewed: 2026-11-04
status: local-station-fixed-dev-validated
---
# 广播程序自助申请 HTTP 契约

本文件记录客户端已实现的自助申请、身份领取及结构化签名协议。公告由本地管理，接口用于身份与受限STATION代签。直接申请、人工审批和CA签证书、发布后自动领取受限凭据的流程保留。

| 操作 | 路径前缀 /api/v1/server-broadcast | 认证 |
| --- | --- | --- |
| POST | /enrollments | 程序预先生成并私有持久保存的fmba1回执 |
| GET | /enrollments/{enrollmentId} | 同一申请的fmba1 |
| POST | /enrollments/{enrollmentId}/credentials | 同一fmba1，正文仅新的领取requestId |
| GET | /identities/{broadcastId} | 自动领取的fmb1机器凭据 |
| POST | /identities/{broadcastId}/station-signatures | 同一fmb1，正文仅requestId/station/online/peak，station为完整本地快照 |

新申请只含requestId、callsign、label、purposeNote、applicant，不含station。applicant包含姓名、邮箱及身份归属/资格/用途说明；label为身份展示名，不是运行公告配置。客户端不提交公钥或身份私钥。首次POST前完整申请与fmba1一并原子保存；同ID同正文恢复申请。申请回执不是设备私钥，不能签STATION或调用管理接口。

领取先保存独立requestId，响应丢失同ID恢复相同token。token、期限和领取状态一起原子保存0600后才使用；窗口结束收到credential_delivery_expired再换新ID，credential_recovery_pending遵守Retry-After。正常90天凭据提前一天续领；若凭据本身不足一天，使用剩余寿命的一部分作为提前量，避免刚领到就不断轮换。暂停或撤销返回403时停止，不以新ID绕过。

状态响应移除profileVersion；身份响应移除station和profileVersion；签名响应移除profileVersion。客户端按本次请求station/online/peak重建16项TBS，验SIG及身份/时间绑定后，使用同一已验证CERT和请求快照本地构造标准APRS行。服务端返回的aprsLine保留为诊断字段，不做相等性要求，也不用于发送。位置符号I后紧接FMO-V4，无空格。国家范围、UID、两种证书哈希、480字节限制和发送期限仍严格检查。不改已签字段，不发历史积压。wait只轮询、自动领取并验链，不连接APRS；once会请求消息签名但默认不发送。

2026-11-05补充对齐上游server-broadcast-signature-only-change.md：响应解码显式接受无aprsLine的新十字段模型，以及带可选字符串aprsLine的旧模型；诊断文本始终忽略。其余十个字段仍必填，未知字段、错误类型或null仍拒绝。签名验真及本地组包无须调整，请求/身份/凭据不变。客户端同时兼容两种响应，不依赖aprsLine。

公告参数由本地管理，配置在进程启动时加载，修改后重启；下次新请求使用新UUID。超时/断线/重启始终重试原ID和持久化的完整正文，不能套用新配置。signature_expired或signature_window_ending明确结束旧请求后才换新ID；不处理profile_changed，不在422后回退旧协议。

待签快照同时持久保存broadcastId及完整公开身份/CERT/证书链与两种哈希，首次POST前原子写入0600状态。恢复先重新检查当前授权，再用原CERT重验链/期限/CRL、验证旧结果；不能因身份轮换替换原快照。旧serverTime不代表当前时间或授权。缺少CERT、错配、吊销或过期时停止；恢复后旧公告丢弃，下一新请求才选新证书。

切换启动时只作废旧expectedProfileVersion待签请求；原申请快照（包括历史station）保留，仅允许用原回执恢复原申请记录，不用作公告配置。证书、UID关联、凭据、CRL及发送防重记录保留。新旧接口不兼容，须统一切换；固定dev已完成该切换与现有身份联合测试。

可信根须预置或管理员明确导入。2026-11-04用户明确产品策略：issuingCountries=[]表示不限国家，非空列表仍严格限制；与当前iOS实现对齐。此策略与官方文档要求非空的写法有差异，记录为明确产品约定，不能通过修改已签证书字段来模拟。

固定dev原入口为http://localhost:18105；生产保持HTTPS。机器token与申请回执均不写日志、URL或普通配置，保存在私有state中。旧已存在的手工token文件仅供原已绑定身份兼容使用，不用于创建新申请或找回缺失回执。现有同呼号/端点不构成接管依据。
