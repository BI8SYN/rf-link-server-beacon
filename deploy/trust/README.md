# 公开信任根

两份证书均为公开Root，无私钥。2026-11-04 本地复验自签名通过；不意味着已检查最新CRL或批准将未来未知Root自动加入信任。

| 文件 | SN | TBS SHA-256/Base64URL |
|---|---|---|
| roots/bg5esn.json | 1 | HRvw8Hu9Xphc08Cuotso9FzDpAIC8djYORwu7KzEDrM |
| roots/rf-link-root-production-v1.json | 8569869802686805 | lPu5cTZM9fOEgCw7nM8GjFPsdV7z8TXEU1QWHOytzow |

文件SHA-256（与证书指纹不同）：

```text
d59c1342f7b1d2fc0791537d1454a69f0d4c14db870c7ce01d26f35c8e524999  roots/bg5esn.json
3b253cd8124342104f997b817574ff071d8f40c9a7638a012632296622722917  roots/rf-link-root-production-v1.json
```

模板SAS配置使用 /home/app/.sas/roots。完整栈的sas-init将本目录公开根复制到SAS命名卷并保留额外根；Beacon单独部署时不修改已有SAS。
