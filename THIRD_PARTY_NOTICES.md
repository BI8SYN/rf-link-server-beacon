# 第三方组件

本项目自身代码采用 MIT，不改变第三方组件的许可。

- `filippo.io/edwards25519` v1.1.0：BSD-3-Clause，源码及原许可位于 `vendor/filippo.io/edwards25519/LICENSE`，用于公钥规范性与子群检查。
- [FMO SAS](https://github.com/BG5ESN/fmo-server-authrozier-service)：独立上游项目，GPL-3.0；本项目 Compose 使用官方构建镜像，不包含或修改其源码。源码、许可证及构建方式请见上游仓库。
- [EMQX](https://github.com/emqx/emqx/tree/v5.8.9)：独立 Broker，具体镜像和对应版本以其 [LICENSE](https://github.com/emqx/emqx/blob/v5.8.9/LICENSE) 及分发内容为准。
- `deploy/trust/roots/` 与程序内 RF Link 根为公开证书资料，不含私钥；来源和指纹见该目录说明。
- 协议公开历史测试向量用于互操作验证，来源见 `docs/protocol/fmo-v4-station.md`。

Go 工具链、官方容器基础镜像及其系统组件按各自许可证分发。固定版本与摘要用于可复现部署，不表示本项目重新授权上游组件。
