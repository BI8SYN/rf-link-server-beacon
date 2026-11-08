---
last-reviewed: 2026-11-07
status: superseded-by-single-sas
---
# 三种部署场景实施计划

> 历史方案，已由[两场景单SAS方案](2026-11-07-single-sas.md)替代，不作为部署操作依据。

已由所有者确认三种业务场景：new-server新建、coexist保留FMO并新增RF Link、convert将FMO转为RF Link。运行方式单独选择；不能因发现原SAS容器就授权替换它。

1. app/CLI增加强制scenario、兼容模式旧端口、Broker网络和认证URL参数。先测试无场景/相同端口在联网前失败、各场景输出隔离，保留验链/CRL及只读测试。
2. 宿主机脚本先选场景再选运行方式；非交互明确参数。新建默认完整栈，兼容默认独立SAS Compose，转换才检测原SAS并提供更新步骤。
3. 增加compose.sas.yaml和可选外部Docker网络覆盖；RF Link SAS使用独立卷/HTTP端口，不创建Broker。生成新监听器HOCON，限定EMQX 5.8.9已验证版本，原FMO监听/全局认证不变。其他版本须另验。
4. 用隔离Docker EMQX和两个受控HTTP认证后端测试旧/新端口正向、跨身份/匿名拒绝、各后端故障不影响另一端口、双端口MQTT消息和ACL拒绝、撤回RF Link入口后FMO仍可用。合成后端测试不等同真实FMO证书登录验收。
5. 三份短教程共用申请/等待/doctor/start步骤，分别强调新建、双入口和备份后转换。更新项目状态与验收边界。
6. 全量race/vet/脚本/所有Compose检查及实际镜像构建；BI8SYN身份提交推送当前分支，原PR #1已合并，改由新PR #2承接，等待push/PR CI通过。不操作生产CA、远程SAS或真实APRS。
