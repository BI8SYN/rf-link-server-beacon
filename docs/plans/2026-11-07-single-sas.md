---
last-reviewed: 2026-11-07
status: completed
---
# 两场景与单 SAS 实施计划

用户已确认仅保留：完全新建 RF Link；已有 FMO 信任并追加 RF Link 广播。兼容场景保留主身份，以同 SAS 的 Mqtt.Clusters 添加新身份，共用 MQTT 地址和端口。删除转换入口和双 SAS/新监听器资源，不自动修改任何运行中的服务器。

## 实施

- [x] 先写失败测试：兼容生成同端口 Cluster、按部署类型给指令；convert/旧监听器参数在联网前拒绝。
- [x] 新增离线候选配置工具 tools/sas-config.py：仅添加一个 Cluster，保留所有原字段；重复运行幂等，UID冲突、错端点、歧义键、无效结构、已有输出文件均失败；原文件永不改写。合成测试覆盖管理员/根/策略/其他节点不变。
- [x] 简化 app/CLI/beacon.sh 场景与部署检测，兼容输出候选生成和对应应用提示；Docker原配置从实际卷或容器复制，不猜测活跃配置路径。
- [x] 删除双 SAS Compose、监听器模板和旧合成双后端测试；用实际 SAS 容器加载合成多身份候选，验证主身份及额外身份均加载。无生产签名或广播。
- [x] README、需求、架构、运维、交接及CI统一两场景；旧计划标记失效，转换文档删除；AGENTS/CLAUDE同步。
- [x] go test -race ./...、go vet ./...、go mod verify、shell语法、合成参数Compose config、Python测试、容器构建与隔离SAS加载测试。记录实际范围，不等同合法设备/语音验收。

## 约束与复核

保留原主身份及管理员；所有身份共用根、issuer限制与认证策略，新增信任意味着扩展同SAS信任域。安装公开根到实际Trust.RootsDir，非空AllowIssuerSn需明确追加允许的签发者，不能清空旧限制。证书轮换仅更新对应Cluster指纹。只生成候选，管理员备份、核对差异并重启原SAS，Broker端口及认证回调不变。容器中的配置必须写回持久卷，不能依赖临时容器层。部署类型识别有歧义时明确选择，不挂载Docker socket。
