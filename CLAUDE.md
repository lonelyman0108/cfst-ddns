# cfst-ddns

Go 后端（`cmd/`、`internal/`）+ Vue 3 / shadcn-vue 前端（`web/`，pnpm），go:embed 打包成单二进制。计划与进度见 `docs/PLAN.md`，接口契约见 `docs/API.md`。

## Git 规范

完整规范见 `CONTRIBUTING.md`，要点：

- 提交信息必须是约定式提交：`<类型>[(作用域)][!]: <中文描述>`，类型限 `feat fix perf refactor docs build ci chore test style revert`。
- 描述写使用者能看懂的中文短句、不加句号——它会直接进入 CHANGELOG。纯 CI、工具、测试改动用 `ci` / `chore` / `test`，避免污染 CHANGELOG。
- 不兼容改动用 `!` 或 `BREAKING CHANGE:` footer；不要随意使用，它会升主版本号。
- 不要直接向 `main` 提交或推送；在功能分支上工作，命名为 `<类型>/<简述>`。
- 不要手动编辑 `CHANGELOG.md`、`.release-please-manifest.json` 或手动打 tag，这些由 release-please 管理。

## 验证

- 后端：`go vet ./... && go test ./...`
- 前端：`cd web && pnpm install --frozen-lockfile && pnpm build`（含 vue-tsc 类型检查）
- 工作流：改动 `.github/workflows/` 后用 actionlint 检查
