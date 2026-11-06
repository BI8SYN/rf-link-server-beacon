---
last-reviewed: 2026-11-06
---
# 用广播身份设置SAS

依据[SAS官方README](https://github.com/BG5ESN/fmo-server-authrozier-service/blob/main/README.md)的CLI、配置路径和Compose说明，以及[官方建服教程](https://bg5esn.com/docs/fmo-server-construction/)中的小写sas命令。

首次使用先按[README四步流程](../../README.md#已有服务器四步启用广播)申请并等待身份就绪，再运行：

```sh
./beacon.sh sas-command
```

命令以只读方式取得当前公开身份，验证证书链、UID/国家/期限、两种哈希及CRL后生成代码。它不会竞争守护进程的状态锁，不保存新状态、不续领凭据、不请求STATION签名、不发APRS，也不修改SAS。当前凭据不可用时明确失败，等待广播程序完成正常续领。

## 按实际部署输出

默认由宿主机脚本识别运行中的官方SAS容器（镜像和Compose标签），以及活动的原生fmo-sas服务或已安装sas二进制；不根据Beacon运行方式推断，不读取Docker环境变量或挂载Docker socket。

| 识别结果 | 只输出 |
|---|---|
| 本项目完整服务器 | 六项.env字段和启动步骤 |
| 其他Compose | 原SAS服务的command与重建步骤 |
| 独立Docker容器 | 镜像之后的启动参数与原卷复用说明 |
| 原生SAS | 原生参数、系统服务应用方式及认证回调 |

未安装、已停止的容器、自定义镜像、远程SAS或多种方式并存可能无法唯一识别。交互终端会询问部署方式；重定向输出或脚本调用时必须明确指定，避免猜错：

```sh
./beacon.sh sas-command --deployment full-stack
./beacon.sh sas-command --deployment compose
./beacon.sh sas-command --deployment docker
./beacon.sh sas-command --deployment native
```

参数选择的是**SAS**，与Beacon是否在Docker运行无关。Go二进制若直接执行，也须明确传入--deployment；自动识别仅在宿主机beacon.sh进行。`--deployment`不会改变实际部署或应用配置，也不自动更新.env。完整服务器首次部署见[教程](full-server.md)。

UID、基础呼号、用户证书指纹、issuerSn取已验证身份；MQTT公开地址/端口取本地配置。SSID不进入server.callsign。用户证书指纹不能换成Root或Intermediate指纹。自定义SAS入口、网络与服务名以实际部署为准。

## 网络部署区别

| SAS位置 | EMQX位置 | SAS监听与回调 |
|---|---|---|
| 原生宿主机 | 同一宿主机原生 | 127.0.0.1:8080；回调http://127.0.0.1:8080/auth |
| 容器 | 同一Docker网络的另一容器 | 容器内0.0.0.0:8080；回调http://sas:8080/auth，sas为实际服务名 |
| 容器 | 宿主机原生 | 容器内0.0.0.0；可仅发布127.0.0.1:8080:8080，原生EMQX回调宿主机127.0.0.1 |
| 原生宿主机 | 容器 | SAS监听容器可达的宿主机地址，EMQX回调该地址；localhost指容器自己 |

0.0.0.0是监听地址，不是回调目标。SAS无需向公网开放8080；跨机器部署应限制为Broker可访问的网络。Beacon在容器运行，不代表实际SAS也在同一台主机或同一网络，因此生成器不自动猜测/更改SAS拓扑。

## 持久配置与Root

手工原生运行默认使用运行用户的~/.sas/config.json及roots；官方安装器的Linux路径可能是/etc/fmo-sas/config.json，应以实际systemd启动参数为准，必要时明确--config。Docker官方卷为sas-data:/home/app/.sas。现有SAS先备份实际配置，将生成的身份及端点合并到原服务配置/Compose，再重启原SAS；不要用全量候选JSON覆盖已有管理员、集群或根信任。

SAS还须安装已认可的RF Link公开Root并保留所需官方Root。当前RF Link Root的TBS指纹为lPu5cTZM9fOEgCw7nM8GjFPsdV7z8TXEU1QWHOytzow；此值只用于Root核验，不填入--cert-fingerprint。原生roots目录属于SAS运行用户；Docker roots在实际持久卷的/home/app/.sas/roots，不是Beacon的/data目录。修改后需另外验证合法客户端登录、目标身份和拒绝路径。

新建服务器可直接使用[完整栈教程](full-server.md)，已有服务则合并生成代码后重启原 SAS。
