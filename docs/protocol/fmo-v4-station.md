---
last-reviewed: 2026-11-04
---
# FMO V4 STATION 实现要点

权威格式：[官方 APRS 格式](https://bg5esn.com/docs/fmo-aprs-formate/)。实现前再次核对上游；此文总结不替代协议原文。

```text
CALL-SSID>APFMO4,TCPIP*:=2505.12N/10242.22EIFMO-V4,STATION,CERT:<blob>,CN,<name>,<host>,P1883,F500KM,U0/0,SIG:<signature>
```

固定表码 `/`、符号 `I`，`FMO-V4`必须紧接符号，不插入空格。2026-11-05发现早期客户端及Trust构造器都误插ASCII空格，虽然双方验签/回传能通过，App严格解析会先报invalidProtocol；新增独立于行构造器的字节前缀回归测试。历史向量的 `F/i` 仅用于验证签名字节，不能直接重发。位置是 APRS 原始度分文本，进入签名时必须逐字一致。文本 UTF-8 不含逗号/CRLF。本接口完整行不含CRLF最多480 UTF-8字节；服务器追加 qAS 路径也要留余量。

CERT 是确定性 CBOR 10元素数组再无 padding Base64URL：

```text
["FMO",4,"userCert",issuerSn,callsign,uid,publicKeyBytes32,iat,exp,caSignatureBytes64]
```

certFingerprint = Base64URL(SHA256(CBOR(前9元素)))；certBlobHash = SHA256(完整10元素CERT原始CBOR)。后者进入公告签名，不得用前者替代。

STATION SIG 的16元素确定性 CBOR：

```text
["FMO",4,"STATION",callsign,ssid,latStr,lonStr,certBlobHashBytes32,
 cc,name,url,port,filterKm,online,peak,timeSalt]
```

timeSalt=floor(UTC Unix秒/600)。接收端可校验当前±1桶；发送端必须新签当前桶。签名 Ed25519，不是直接签 JSON、APRS 文本，也不是 Ed25519ph。CERT 自身签名与 SIG 是两个签名。

验证：固定域分离 → 头呼号与证书一致（忽略SSID）→ Root/Intermediate/User链签名和issuer绑定 → 三层时间及国家/UID范围 → CRL → 公告SIG及时间桶。仅更换包头不能复用另一呼号证书。

APRS-IS 登录需合法获准使用的呼号/passcode，获得 verified 响应后才发送；不是把 FMO 证书当 APRS 登录密码。APRS 收到回传只证明传播，不能证明盒子已验签或显示。

STATION 当前没有 usesTLS 字段；配置可以保存该布尔值，但标准线上格式不得擅自增加字段或按端口猜 TLS。若产品要广播TLS能力，另做兼容性设计。
