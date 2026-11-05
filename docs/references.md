---
last-reviewed: 2026-11-05
---
# 参考资料

- [FMO 官方建服教程](https://bg5esn.com/docs/fmo-server-construction/)：原生 SAS、认证回调、服务器部署。
- [官方 FMO APRS 协议](https://bg5esn.com/docs/fmo-aprs-formate/)：STATION、CERT/SIG、十六项 CBOR 与 600 秒广播建议。
- [SAS 源码与 Compose](https://github.com/BG5ESN/fmo-server-authrozier-service)：CLI、根加载、HTTP /auth 与持久目录。模板固定 1.0.6 镜像及摘要。
- [FAS 教程](https://bg5esn.com/docs/fmo-fas-install-guide/)：可选消息审计。
- [EMQX 官方文档](https://docs.emqx.com/en/emqx/v5.8/access-control/authn/http.html)：HTTP 认证器与结果处理。
- [Go 官方版本](https://go.dev/dl/)：构建工具链。
- 本项目客户端的 HTTP 契约独立保存在 [remote-signer-v1.md](contracts/remote-signer-v1.md)，无需访问开发者本地项目路径。

根证书来源和指纹见 [deploy/trust](../deploy/trust/README.md)。HTTPS 系统 CA 与 FMO 证书信任根是两套独立机制。
