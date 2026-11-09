---
last-reviewed: 2026-11-09
---
# 原生预编译安装

用户已授权：GitHub CI构建Linux amd64/arm64预编译文件；deploy-native自动下载，无需Go，测试后BI8SYN提交推送并确认CI。

实现：下载固定仓库Release的二进制与SHA256SUMS，校验成功才替换本地候选；失败保留旧程序与服务。自动识别架构，未知架构失败；可用BEACON_VERSION固定版本。GitHub Actions每次CI交叉构建两种架构并保存artifact，版本tag发布同样资产至Release。现有systemd安装与数据保护保持。公开首版版本v0.1.0，CI通过后创建tag由Actions发布。

验证：离线模拟下载、校验失败保留旧程序、架构和版本拒绝；Go/race/vet、shell、Compose、跨架构构建、GitHub CI及Release资产验证。不操作生产签名或广播。
