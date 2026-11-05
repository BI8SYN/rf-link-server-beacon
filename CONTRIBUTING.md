# 参与开发

欢迎通过 Issue 描述问题、通过 Pull Request 提交改进。请附操作系统、版本、部署方式与脱敏错误；不要上传 data/、申请回执、机器凭据、APRS passcode、私钥或完整身份载荷。

文档使用中文，代码标识符使用英文。使用短期 `feature/` 或 `fix/` 分支和 Conventional Commits；PR 说明行为变化、测试与限制。修改配置、接口或部署行为时同步文档。

提交前运行 `go test -race ./...`、`go vet ./...`、`go mod verify` 和 `sh -n beacon.sh`。Compose 检查需提供合成环境参数，不能用 CI 申请真实身份、调用生产签名或发 APRS。联网集成测试必须由操作人明确启用并使用自己的身份。

生产 CA 签发属于持有人操作，不是项目自动化任务。根证书更新需核验自签名和 TBS 指纹，未知根不能自动授信。
