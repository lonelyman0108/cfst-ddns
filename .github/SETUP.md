# GitHub Actions 设置指南

本项目使用 GitHub Actions 自动构建和推送多架构 Docker 镜像到 Docker Hub。

## 配置步骤

### 1. 创建 Docker Hub 账号

如果还没有 Docker Hub 账号，请前往 [Docker Hub](https://hub.docker.com/) 注册。

### 2. 配置 GitHub Secrets

在你的 GitHub 仓库中配置以下 Secrets：

1. 进入仓库 Settings → Secrets and variables → Actions
2. 点击 "New repository secret" 添加以下两个 secrets：

#### DOCKER_USERNAME
- **Name**: `DOCKER_USERNAME`
- **Secret**: 你的 Docker Hub 用户名（例如：`lonelyman0108`）

#### DOCKER_PASSWORD
- **Name**: `DOCKER_PASSWORD`
- **Secret**: 你的 Docker Hub 访问令牌（Access Token）

> **注意**：强烈建议使用 Access Token 而不是密码
>
> 获取 Access Token：
> 1. 登录 Docker Hub
> 2. 点击右上角头像 → Account Settings → Security
> 3. 点击 "New Access Token"
> 4. 输入描述（如 "GitHub Actions"），选择权限（Read & Write）
> 5. 点击 "Generate" 并复制生成的 token
> 6. 将 token 粘贴到 GitHub Secret 中

### 3. 触发规则

| 触发 | 运行内容 |
|---|---|
| Pull Request（目标 `main`，忽略纯文档改动） | 前端构建 + `go vet` + `go test`（不推送镜像） |
| 推送 / 合并到 `main` | 测试 + release-please 更新 Release PR |
| 合并 Release PR | 打 `v*` tag 并创建 Release → 多平台二进制 → 推送 standard/bundled 镜像 → 上传二进制到 Release |
| 手动推送 `v*` 标签（兜底） | 同上，Release 说明由 GitHub 自动生成 |
| 手动触发（Actions 页面 Run workflow） | 测试 → 多平台二进制（Artifact）→ 推送 `dev` / `dev-bundled` 镜像 |

功能分支的推送不会触发构建；需要验证时开 PR 到 `main` 或手动触发。同一 PR 的新提交会取消旧的运行。

### 4. 镜像标签说明

| 触发条件 | standard 标签 | bundled 标签（预置 cfst） |
|---|---|---|
| 发布 `v2.0.0` | `2.0.0`, `2.0`, `2`, `latest` | `2.0.0-bundled`, `2.0-bundled`, `2-bundled`, `latest-bundled` |
| 手动触发 | `dev`, `dev-<sha>` | `dev-bundled`, `dev-bundled-<sha>` |

### 5. 支持的架构

工作流会自动构建以下架构的镜像：

- `linux/amd64` - x86_64 架构（Intel/AMD 处理器）
- `linux/arm64` - ARM 64位架构（树莓派 4、Apple Silicon 等）
- `linux/arm/v7` - ARM v7 架构（树莓派 3 等）

Docker 会根据你的系统自动选择合适的架构。

### 6. 发布新版本

版本号与 CHANGELOG 由 [release-please](https://github.com/googleapis/release-please) 根据**约定式提交**自动生成，无需手动打 tag。提交与 PR 规范见 [CONTRIBUTING.md](../CONTRIBUTING.md)。

1. 功能分支开 PR 合并到 `main`（Squash merge，PR 标题即提交信息）。
2. release-please 自动创建或更新 `chore(main): release x.y.z` PR，内含 CHANGELOG 与版本号改动。
3. 想发版时合并这个 Release PR，工作流会自动打 tag、创建 Release、构建二进制与镜像。

需要手动指定版本号时，在合并提交信息的**最后一行**加 `Release-As: x.y.z`（只影响下一次发版）。

#### 仓库设置（一次性）

| 位置 | 设置 |
|---|---|
| Settings → Actions → General → Workflow permissions | 勾选 **Allow GitHub Actions to create and approve pull requests**（release-please 需要） |
| Settings → General → Pull Requests | 只保留 **Allow squash merging**，默认提交信息选 **Pull request title**；勾选 **Automatically delete head branches** |
| Settings → Rules → Rulesets（目标 `main`） | 勾选 **Require a pull request before merging**、**Require status checks to pass**（`test`、`Validate PR title`）、**Block force pushes**；Bypass list 加入 **Repository admin** |

> Release PR 由 `GITHUB_TOKEN` 创建，不会触发任何 workflow，因此必需的状态检查永远不会出现。合并它时需以管理员身份绕过规则（合并按钮下选择 bypass）。它只改 CHANGELOG 和版本号，合并后 `main` 上会照常测试。

### 7. 查看构建状态

1. 进入仓库的 "Actions" 标签页
2. 查看 "Build and Release" 工作流
3. 点击具体的运行记录查看详细日志

### 8. 故障排查

#### 构建失败
- 先确认 test 任务通过（前端 `pnpm build` 与 `go test`）
- 检查 Dockerfile 语法是否正确
- 查看 Actions 日志中的错误信息
- 确认依赖项是否可用

#### 推送失败
- 验证 `DOCKER_USERNAME` 是否正确
- 确认 `DOCKER_PASSWORD` 中的 Access Token 是否有效且具有写入权限
- 检查 Docker Hub 仓库名称是否正确（在 workflow 文件中的 `DOCKER_IMAGE` 变量）

#### 镜像拉取慢
- 可以使用国内的 Docker 镜像加速服务
- 或直接从 GitHub Packages 拉取（需要额外配置）
