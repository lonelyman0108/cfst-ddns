# 贡献与发布规范

## 分支

- `main`：始终可发布，**不直接推送**，所有改动走 Pull Request。
- 功能分支从 `main` 拉出，命名为 `<类型>/<简述>`，如 `feat/huawei-dns`、`fix/dingtalk-sign`。
- 功能分支的推送不触发 CI；需要验证时开 PR（可以是 Draft PR）。

## 提交信息

采用 [约定式提交](https://www.conventionalcommits.org/zh-hans/v1.0.0/)，版本号与 CHANGELOG 都由它自动生成：

```
<类型>[(作用域)][!]: <描述>

[正文：为什么改、怎么改]

[footer：BREAKING CHANGE: ... / Release-As: x.y.z]
```

| 类型 | 用途 | 版本号 | CHANGELOG 分组 |
|---|---|---|---|
| `feat` | 新功能 | 升次版本号 | 新功能 |
| `fix` | 修复 bug | 升补丁号 | 问题修复 |
| `perf` | 性能优化 | 升补丁号 | 性能优化 |
| `refactor` | 重构，不改变行为 | 升补丁号 | 重构 |
| `docs` | 文档 | 升补丁号 | 文档 |
| `build` | 构建、Dockerfile、依赖 | 升补丁号 | 构建 |
| `ci` / `chore` / `test` / `style` / `revert` | CI、杂项、测试、格式、回滚 | 不单独触发发版 | 不展示 |

- **不兼容改动**：类型后加 `!`（如 `feat!: 配置存储改为 PostgreSQL`），或在 footer 写 `BREAKING CHANGE: 说明`，升主版本号。
- **描述**用中文，写使用者能看懂的一句话，不加句号；它会原样出现在 CHANGELOG 里。
  - ✅ `feat: 支持华为云 DNS`
  - ❌ `feat: add huawei provider impl`、`fix: 修复bug`
- **作用域**可选，建议使用模块名：`api`、`web`、`engine`、`provider`、`notify`、`cfst`、`docker`。
- 标题行不超过 100 字符，细节写在正文。

### 本地校验

仓库自带 `commit-msg` 钩子，克隆后执行一次即可启用：

```bash
git config core.hooksPath .githooks
```

## Pull Request

- **PR 标题就是最终的提交信息**（Squash merge），必须符合上面的格式，CI 会检查。
- 合并前需通过测试（前端构建 + `go vet` + `go test`）。
- 统一使用 **Squash and merge**，一个 PR 对应 CHANGELOG 里的一条记录。

## 发布

无需手动打 tag 或编辑 CHANGELOG：

1. PR 合并到 `main` 后，release-please 自动创建或更新 `chore(main): release x.y.z` PR。
2. 想发版时合并该 Release PR，自动打 tag、创建 Release、构建多平台二进制和 Docker 镜像。

需要指定版本号时，在合并提交信息的**最后一行**加 `Release-As: x.y.z`，只影响下一次发版。

CI、镜像标签与仓库设置详见 [.github/SETUP.md](.github/SETUP.md)。
