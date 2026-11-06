# RF Link Server Beacon 协作说明

中文交流和文档；代码标识符使用英文。先读 README.md、docs/project-brief.md、docs/handoff.md。不自动派发子 agent，除非用户要求。

## 文档地图

| 用途 | 路径 |
|---|---|
| 上下文与交接 | docs/project-brief.md、docs/handoff.md |
| 需求与技术约束 | docs/spec/product-spec.md、docs/spec/technical-spec.md |
| 架构与决策 | docs/architecture/overview.md、docs/adr/README.md |
| HTTP与协议 | docs/contracts/remote-signer-v1.md、docs/protocol/fmo-v4-station.md |
| 安装与服务器 | docs/operations/deployment.md、docs/operations/full-server.md、docs/operations/coexist.md、docs/operations/convert.md、docs/operations/sas-setup.md |
| 验收 | docs/development/open-source-validation.md |
| 本地技能 | .agents/skills/、.claude/skills/ |

## 安全和权限

- 不创建生产CA或执行生产证书、CRL、Trust Manifest签名；生产CA签名由持有人亲自操作，测试只用临时密钥与fake server。
- 不读取或输出用户生产私钥、seed，不把回执、令牌、APRS passcode或私钥放入Git、镜像、日志和命令行。广播私钥只位于远端签名设施。
- 实际生产申请、签名、APRS发报和远程服务器变更需要用户明确授权。不能因测试而修改现有生产服务。
- 不提供任意bytes签名入口。未知Root不能自动授信；回环HTTP/CRL仅明确的本机dev使用，测试身份不能冒充生产。
- issuingCountries为空代表不限国家，非空严格限制。证书/CRL防回滚、发送防重与待签原CERT/链/哈希必须保留。
- 迁移先停止源实例，再复制完整data；本地历史备份不得运行同身份发送。不得触碰其他项目的Compose。
- 不挂载Docker socket或使用privileged。SAS与EMQX健康检查不等于合法客户端、语音或官方设备兼容性验收。

## 工作流

会话开始按本地doc-drift-check只读检查文档日期，结果仅提示。先更新需求/契约/计划，离线测试，再实现与同步文档。已确认需求不重复追问，未实现功能明确失败。

最低检查为go test ./...、go vet ./...、脚本语法和Compose config；完整栈Compose需合成环境参数。CI不调用生产签名、不联网广播。容器构建与实际运行范围必须如实记录。

AGENTS.md与CLAUDE.md保持一致，本地技能两处镜像同步。可选技能不可阻塞同等人工测试与文档检查。

## Git Conventions

main为稳定公开分支，日常使用feature/或fix/短期分支、Conventional Commits、PR审查与squash。首次公开发布按所有者明确授权直接推送main；后续发布、push和镜像分发须按任务授权执行。维护者BI8SYN的提交使用其账号身份，不混用旧的个人身份。
