---
last-reviewed: 2026-11-04
---
# 模块索引

所有下列包均已实现，测试与源码同目录。CLI负责命令入口，jsonx提供严格JSON解码，location负责网格与APRS坐标转换。

| 包 | 输入 | 输出 / 责任 |
|---|---|---|
| config | JSON/秘密文件路径 | 严格校验的 Config，不联网 |
| identity | 认可的 Root、公开链、CRL | BoundIdentity；指纹、有效性与CRL状态 |
| protocol | BoundIdentity + Station + timeSalt | 确定性 CBOR、CERT、SIG验证、APRS行 |
| signer | 节点凭据 + 请求 | 经 HTTP 获取的公开签名响应，不本地签名 |
| aprs | 已验证的报文 | verified登录、单次发送、重连、状态 |
| app | 上述接口 | 初始化/调度、取消、状态持久化 |

不得在 protocol 中请求网络；不得在 aprs 中创建/修改证书；不得在 app 中绕过 verifier 直接发送 raw HTTP 响应。
