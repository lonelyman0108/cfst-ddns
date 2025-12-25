# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.1.0] - 2024-12-25

### Added
- 添加对 DNSPod (腾讯云) 的支持，更新配置示例和脚本逻辑 (1192c71)
- 添加捆绑版本 Docker 镜像支持，更新文档和工作流 (fa99964)

### Changed
- 将 DNSPod API 从腾讯云 API v3 改为简化的 Token 认证方式 (93b0646)
  - 配置从 `DNSPOD_SECRET_ID` + `DNSPOD_SECRET_KEY` 改为单一的 `DNSPOD_TOKEN`
  - 简化 DNSPod API 调用逻辑，移除复杂的 TC3-HMAC-SHA256 签名
  - 移除对 `jq` 的依赖，使用 `grep` 解析 JSON
  - 修复 DNSPod API 状态码 10（记录列表为空）的处理逻辑
- 优化 GitHub 镜像站点说明，建议配置国内镜像以加速下载 (aa69c9e)

## [1.0.2] - 2024-12-24

### Fixed
- 简化标签生成逻辑，仅在手动触发时添加 dev 标签和 commit SHA 短标签 (0244cd7)

## [1.0.1] - 2024-12-24

### Added
- 添加 CFST_VERSION 环境变量以支持手动指定版本号 (47b66f2)
- 添加日志管理功能，支持日志文件大小限制和自动轮转 (d088e4e)

### Changed
- 更新构建 Docker 镜像和创建发布的工作流 (26e9fb1)

### Fixed
- 修复 docker-compose 下单次模式会无限循环的问题 (d5f2e9a)
- 修改定时任务输出方式，使用 tee 命令以便同时记录日志和输出 (5fab484)
- 更新测速模式说明，调整为根据模式决定 DNS 记录类型 (c8e05c0)
- 注释掉不必要的分支限制，简化工作流触发条件 (87a0841)
- 移除创建发布步骤中的 GITHUB_TOKEN 环境变量 (e95b73f)

## [1.0.0] - 2024-12-24

### Added
- 首次发布 (083e0b7)
- CloudFlare Speed Test (CFST) DDNS 基础功能实现
- 支持自动测速并更新 DNS 记录
- Docker 和 Docker Compose 支持

[Unreleased]: https://github.com/lonelyman0108/cfst-ddns/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/lonelyman0108/cfst-ddns/compare/v1.0.2...v1.1.0
[1.0.2]: https://github.com/lonelyman0108/cfst-ddns/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/lonelyman0108/cfst-ddns/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/lonelyman0108/cfst-ddns/releases/tag/v1.0.0
