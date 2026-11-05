---
last-reviewed: 2026-11-05
---
# 用广播身份设置SAS

依据[SAS官方README](https://github.com/BG5ESN/fmo-server-authrozier-service/blob/main/README.md)的CLI、配置路径和Compose说明，以及[官方建服教程](https://bg5esn.com/docs/fmo-server-construction/)中的小写sas命令。

先取得并发布广播身份，再运行：

```sh
./beacon.sh sas-command
```

命令以只读方式取得当前公开身份，验证证书链、UID/国家/期限、两种哈希及CRL后生成代码。它不会竞争守护进程的状态锁，不保存新状态、不续领凭据、不请求STATION签名、不发APRS，也不修改SAS。当前凭据不可用时明确失败，等待广播程序完成正常续领。

## 生成内容

- 原生初始化/启动命令：UID、基础呼号、用户证书指纹、issuerSn取已验证用户身份；MQTT公开地址/端口取本地配置。SSID不进入server.callsign，服务器名称也不是SAS CLI参数。
- 官方SAS Docker Compose的command数组：保留既有镜像和数据卷，只合并参数；不生成未经核验的镜像版本。官方镜像入口是dotnet sas.dll，Compose的command只填后续参数，不能重复加./sas或在运行中的容器里再启动第二个监听进程。
- 本项目.env的六个身份/MQTT字段（官方模板使用前五项）：保留其他配置及Dashboard密码；旧持久配置另须核对issuerSn及Http.addr，推荐使用显式command参数。

用户证书指纹不能替换成 Root 或 Intermediate 指纹，以命令实时输出为准。

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
