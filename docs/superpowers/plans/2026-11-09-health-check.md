# 优选 IP 健康检测实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为任务增加基于真实 DNS 记录的定时 IP 健康检查，在连续异常后通过现有 cfst 全量测速自动切换，并保存可追溯的检测历史。

**Architecture:** 健康配置保存在 Task，运行状态和每次检测保存在独立的 HealthState/HealthCheck 表。Scheduler 为每个任务维护独立的健康 entry；健康 job 与普通 Run 共用 Engine 串行队列。健康 job 每次从 DNS Provider 读取实际记录，使用 cfst 的 `-ip` 精确测试当前 IP；达到连续失败和冷却条件后再 enqueue 现有完整任务，使用 `trigger=health`。

**Tech Stack:** Go 1.26、GORM + glebarez/sqlite、robfig/cron、CloudflareSpeedTest 子进程、Gin REST API、Vue 3 + TypeScript + shadcn-vue/Tailwind v4 + vue-i18n。

**Spec:** `docs/superpowers/specs/2026-11-09-health-check-design.md`

## Global Constraints

- 健康检查 IP 必须每次从 DNS 服务商实际记录读取，不能用 RecordState 回退后继续检测。
- `proxied=true` 的 Cloudflare Target 跳过检测并提示；同任务其他非代理 Target 继续检测。
- 健康检查只测试当前 IP，不执行全量 IP 段扫描；完整测速才负责重新选优。
- Provider 查询失败记录为 unknown，不增加连续异常、不触发 DNS 修改。
- 健康检查和完整测速必须共用全局串行 Engine 队列。
- 新增前端文案必须同步 `zh-CN`、`zh-TW`、`en`、`ja`。
- 不手动编辑 `CHANGELOG.md`、`.release-please-manifest.json` 或手动打 tag。
- 实现完成前必须通过 `go vet ./... && go test ./...` 与 `cd web && pnpm install --frozen-lockfile && pnpm build`。

## Review Focus

1. DNS API 返回记录与任务线路不匹配时不能误测或误切换；由 Task 3 的线路过滤和 Provider 集成测试覆盖。
2. Provider 暂时失败时必须是 unknown 而非 unhealthy；由 Task 3 的错误路径测试覆盖。
3. 透明代理/HTTPing 模式不能因健康检查误判而触发全量测速；由 Task 3 的网络模式测试覆盖。
4. 健康 job 与 cron/manual Run 不得并发启动 cfst；由 Task 4 的队列串行测试覆盖。
5. 健康配置缺失、旧备份恢复和前端四语言字段不完整不能破坏旧任务；由 Task 1、Task 5、Task 6 的兼容性测试覆盖。

---

### Task 1: 增加健康配置、状态与历史模型

**Files:**
- Modify: `internal/store/models.go`
- Modify: `internal/store/store.go`
- Test: `internal/store/store_test.go`
- Modify: `docs/API.md`

**Interfaces:**
- Produces `store.HealthCheckConfig`, `store.HealthState`, `store.HealthResult`, `store.HealthCheck`。
- Produces Store methods `GetHealthState(taskID uint)`, `SaveHealthState(state HealthState) error`, `CreateHealthCheck(check *HealthCheck) error`, `ListHealthChecks(taskID uint, limit int) ([]HealthCheck, error)`, `PurgeHealthChecks(days int) (int64, error)`。
- `store.Task` 增加 `HealthCheck HealthCheckConfig`，JSON key 为 `healthCheck`。

- [ ] **Step 1: Write failing model/store tests**

在 `internal/store/store_test.go` 增加测试：

- 新建 Task 时缺少健康配置仍能读取零值，并由 API 层默认值补齐。
- `GetHealthState` 对不存在的 task 返回默认状态而不是数据库错误。
- `SaveHealthState` 按 `task_id` upsert，不产生重复行。
- `HealthCheck.Results` JSON 序列化后可完整读回 IPv4、IPv6、proxied 跳过原因和 unknown 原因。
- `PurgeHealthChecks` 只删除过期健康记录，不删除 Run。

- [ ] **Step 2: Run focused tests to verify failure**

Run: `go test ./internal/store -run 'Test(Health|PurgeHealth)' -count=1`
Expected: FAIL because the new types and methods do not exist。

- [ ] **Step 3: Implement models and store methods**

在 `models.go` 增加配置、状态、结果和历史模型；`Task.HealthCheck` 使用现有 GORM JSON serializer。`store.Open` 的 AutoMigrate 增加 `HealthState`、`HealthCheck`。状态使用 `(task_id)` 唯一键 upsert；历史按 `checked_at` 倒序查询，limit 限制为 1–200。为旧数据库和旧备份保留零值兼容。

- [ ] **Step 4: Add retention and API contract documentation**

在 `docs/API.md` 补充健康配置、任务只读状态、`GET /api/tasks/:id/health` 响应、`health` Run trigger 和健康历史清理语义。不要在本任务实现 HTTP handler。

- [ ] **Step 5: Run focused tests**

Run: `go test ./internal/store -run 'Test(Health|PurgeHealth)' -count=1`
Expected: PASS。

### Task 2: 支持 cfst 精确 IP 测速

**Files:**
- Modify: `internal/cfst/runner.go`
- Modify: `internal/cfst/cfst_test.go`

**Interfaces:**
- `cfst.RunOptions` 增加 `IPs []string`；非空时进入精确 IP 模式。
- 新增 `BuildArgsForIPs(c store.SpeedTestConfig, ips []string, outFile string) ([]string, error)`。
- 既有 `BuildArgs` 保持默认 IP 文件模式兼容。

- [ ] **Step 1: Write failing argument and runner tests**

新增测试：

- `BuildArgsForIPs` 对单个 IPv4、多个 IPv4/IPv6 生成 `-ip`，不生成 `-f`，始终带 `-o` 和 `-p 0`。
- 精确 IP 模式保留 `-httping`、端口、URL 和额外参数，但由程序控制的输入/输出参数覆盖冲突值。
- 只延迟健康检查能通过 `-dd` 禁止下载；需要速度时下载数量覆盖为当前 IP 数量。
- `Manager.Run` 的 `RunOptions.IPs` 使用精确 IP 构造，不读取默认 `ip.txt`。

- [ ] **Step 2: Run focused tests to verify failure**

Run: `go test ./internal/cfst -run 'Test(BuildArgs|RunExactIPs)' -count=1`
Expected: FAIL because exact-IP interfaces do not exist。

- [ ] **Step 3: Implement exact-IP mode**

抽取现有参数公共部分，增加 `-ip` 模式；对输入 IP 做 trim、去空、去重和基本 `net.ParseIP` 校验。健康模式的过滤阈值由调用者关闭，runner 只负责返回原始 CSV 结果。继续使用现有临时 CSV、代理环境清理、取消和错误处理逻辑。

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/cfst -run 'Test(BuildArgs|RunExactIPs)' -count=1`
Expected: PASS。

### Task 3: 实现真实 DNS 读取、健康判定和状态机

**Files:**
- Create: `internal/engine/health.go`
- Create: `internal/engine/health_test.go`
- Modify: `internal/engine/engine.go`
- Modify: `internal/provider/provider.go` only if a reusable line-filter helper is needed

**Interfaces:**
- `engine.HealthTargetIP`：包含 `TargetKey`、`AccountID`、`FQDN`、`IPType`、`IP`，用于把去重后的 IP 映射回 DNS Target。
- `engine.HealthJobResult`：包含 `CheckID uint` 和 `TriggerFullRun bool`。
- `engine.HealthJob(ctx context.Context, taskID uint) (HealthJobResult, error)`：执行一次健康检查并按状态更新 Store。
- `engine.CollectCurrentIPs(ctx context.Context, task *store.Task) ([]HealthTargetIP, []store.HealthResult, error)`：从 Provider 实际读取并按线路收集当前 IP；Provider 错误写入 unknown 结果，不能回退 RecordState。
- `engine.EvaluateHealth(results []store.HealthResult, cfg store.HealthCheckConfig) (healthy bool, reason string)`。
- `engine.ShouldTriggerHealth(state store.HealthState, now time.Time, cfg store.HealthCheckConfig, unhealthy bool) bool`。

- [ ] **Step 1: Write failing pure state/threshold tests**

测试 `EvaluateHealth`：只配置延迟、只配置速度、两个阈值同时配置、缺少结果、proxied skipped、unknown 不计入异常。测试 `ShouldTriggerHealth`：未达到连续次数、不在冷却、仍在冷却、冷却时间为空、阈值为 1。

- [ ] **Step 2: Run focused tests to verify failure**

Run: `go test ./internal/engine -run 'Test(EvaluateHealth|ShouldTriggerHealth)' -count=1`
Expected: FAIL。

- [ ] **Step 3: Implement DNS record collection**

遍历 Task Targets 和 `ipTypes(task.IPType)`，构造 Provider，调用 `ListRecords`，使用现有 `sameLine` 语义过滤线路；Cloudflare proxied Target 写入 skipped 结果，不进入 cfst IP 列表。按 TargetKey/IPType/IP 去重，同时保留 Target 映射供历史展示。Provider 查询错误返回 unknown 结果，不回退 RecordState；`CollectCurrentIPs` 返回可检测 IP 映射、已生成的 unknown/skipped 结果以及非 Provider 级致命错误。

- [ ] **Step 4: Implement health probe and state transitions**

对每种 IP 类型调用 Task 2 的精确 IP runner；直连模式复用 `netcheck.Probe`，proxy 模式跳过保留地址判定并沿用 HTTPing。把每个 IP 的原始指标和原因写入 HealthCheck。healthy 清零连续失败，unhealthy 增加连续失败，unknown/skipped 不改变计数；达到阈值且不在冷却时设置 cooldown，并返回需要触发完整 Run 的结果。

- [ ] **Step 5: Add integration tests with fake Provider/fake cfst**

覆盖真实 DNS 返回多目标、多线路、重复 IP、proxied Target、Provider 错误；验证只测试返回的 IP；连续两次异常不触发、第三次触发、冷却内不重复触发；完整 Run 成功更新后清除健康异常，失败不清除；直连代理检测不会触发重选。

- [ ] **Step 6: Run focused tests**

Run: `go test ./internal/engine -run 'Test(EvaluateHealth|ShouldTriggerHealth|Health)' -count=1`
Expected: PASS。

### Task 4: 把健康 job 接入串行队列和 Scheduler

**Files:**
- Modify: `internal/engine/engine.go`
- Modify: `internal/scheduler/scheduler.go`
- Modify: `internal/api/tasks.go`
- Modify: `internal/app/app.go`
- Test: `internal/engine/health_test.go`
- Create or modify: `internal/scheduler/scheduler_test.go` if existing test conventions permit

**Interfaces:**
- Engine 内部队列从 `chan uint` 改为带 `kind/taskID/runID` 的 job；普通 `Enqueue` 行为保持不变。
- 新增 `Engine.EnqueueHealth(taskID uint) error`，同一任务健康 job 去重。
- Scheduler 增加健康 entry，并在 `Sync`/`Remove`/`Start` 中同步。
- `taskView` 增加 `HealthStatus` 和 `HealthNextRunAt`。

- [ ] **Step 1: Write failing queue/scheduler tests**

测试：健康 job 和普通 Run 不会同时进入 cfst；同一任务健康 job 不重复排队；普通 Run 已排队/运行时健康 job 被 skipped；`@every <minutes>m` 随任务新增、修改、禁用、删除正确注册/移除；健康触发的完整 Run 的 `Trigger` 为 `health`。

- [ ] **Step 2: Run focused tests to verify failure**

Run: `go test ./internal/engine ./internal/scheduler -run '(Health|Scheduler)' -count=1`
Expected: FAIL。

- [ ] **Step 3: Refactor Engine worker and wire health jobs**

保留现有任务级普通 Run 去重和取消语义，增加健康 job 去重集合。worker 根据 job kind 调用普通执行或 `HealthJob`；健康 job 返回 `HealthJobResult.TriggerFullRun=true` 后再调用现有 `Enqueue(taskID, "health", false)`，不能在 worker 中同步嵌套执行完整 Run。

- [ ] **Step 4: Add health scheduler entries and task defaults/validation**

任务默认健康检查关闭；normalize 时校验间隔、阈值、连续次数、冷却时间。Scheduler 使用 robfig/cron 的 `@every` descriptor，任务禁用或健康关闭时移除 entry。API task view 返回健康状态和下次健康检查时间。

- [ ] **Step 5: Run focused tests**

Run: `go test ./internal/engine ./internal/scheduler ./internal/api -run '(Health|Scheduler|Task)' -count=1`
Expected: PASS。

### Task 5: 增加健康 API、备份和历史清理

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/api/tasks.go`
- Modify: `internal/api/system.go`
- Modify: `internal/api/runs.go`
- Modify: `internal/engine/notify.go` if trigger label/message needs a dedicated branch
- Modify: `internal/store/store.go`
- Test: `internal/api/*_test.go` or existing API integration test location

**Interfaces:**
- `GET /api/tasks/:id/health?limit=50` 返回 `{state, items}`。
- Task create/update 接受并返回 `healthCheck`；Task view 返回 `healthStatus` 和 `healthNextRunAt`。
- Run trigger 类型增加 `health`，旧客户端仍能读取其它字段。

- [ ] **Step 1: Write failing API tests**

验证任务默认值、非法配置返回 400、健康接口按时间倒序且 limit 最大 200、未初始化健康状态有稳定默认值、`health` Run 正确返回。验证备份恢复旧备份没有健康字段时仍成功，含健康表数据时事务恢复完整。

- [ ] **Step 2: Run focused tests to verify failure**

Run: `go test ./internal/api ./internal/store -run '(Health|Backup|Task)' -count=1`
Expected: FAIL。

- [ ] **Step 3: Implement handlers and backup/retention integration**

注册受认证保护的健康历史路由；复用现有 `bind`、`fail`、`dbFail` 和语言处理。备份/恢复纳入健康配置、HealthState、HealthCheck；每日 purge 与手动 purge 同时清理健康历史。健康检查自身不走通知，健康触发的普通 Run 复用现有通知条件。

- [ ] **Step 4: Run focused API tests**

Run: `go test ./internal/api ./internal/store -run '(Health|Backup|Task)' -count=1`
Expected: PASS。

### Task 6: 实现前端配置、状态和历史展示

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/tasks.ts`
- Modify: `web/src/views/TaskEditView.vue`
- Modify: `web/src/views/TaskListView.vue`
- Modify: `web/src/views/RunListView.vue`
- Modify: `web/src/views/RunDetailView.vue`
- Modify: `web/src/locales/zh-CN/tasks.ts`, `web/src/locales/zh-TW/tasks.ts`, `web/src/locales/en/tasks.ts`, `web/src/locales/ja/tasks.ts`
- Modify: corresponding `runs.ts`, `messages.ts`, `common.ts` locale files as needed

**Interfaces:**
- TypeScript `HealthCheckConfig`, `HealthStatus`, `HealthResult`, `HealthCheckHistory`, `RunTrigger` 增加 `health`。
- `tasksApi.health(id, limit)` 调用 `/api/tasks/:id/health`。

- [ ] **Step 1: Add type/API tests or compile-first fixtures**

补齐接口类型和 API 方法，确保旧 Task 缺少 `healthCheck` 时前端使用默认值，不出现 undefined 控件状态。

- [ ] **Step 2: Implement task editor**

在测速参数附近增加健康检查 Card/Section：启用、间隔、延迟上限、最低速度、连续异常次数、冷却时间。至少一个阈值校验由前端先提示，后端仍是最终校验。proxied Target 的健康跳过说明直接显示，不用 tooltip。

- [ ] **Step 3: Implement list/detail/history display**

任务列表显示最近健康状态、连续失败/冷却提示；任务编辑或任务详情可查看最近检测记录，每条记录显示 IP、类型、延迟、速度、状态、原因和 proxied 跳过。Run 列表/详情显示“健康检查触发”。处理 IP 地址换行、窄屏隐藏次要列和长文案溢出。

- [ ] **Step 4: Add all four locale translations**

四种语言键保持一致；健康配置帮助文字、状态、unknown、proxied skipped、触发原因、运行 trigger 和校验错误全部覆盖。新增英文/日文长文案检查布局。

- [ ] **Step 5: Build frontend**

Run: `cd web && pnpm install --frozen-lockfile && pnpm build`
Expected: vue-tsc 和 Vite 构建通过，locale coverage 无键缺失。

### Task 7: 全量验证与文档收尾

**Files:**
- Modify: `docs/PLAN.md`
- Modify: `docs/API.md` if implementation details changed during tests
- Modify: `README.md`, `README.zh-CN.md`, `README.zh-TW.md`, `README.ja.md` only if user-facing setup instructions need the new feature
- Test: all existing Go/frontend tests

- [ ] **Step 1: Run backend verification**

Run: `go vet ./... && go test ./...`
Expected: no vet diagnostics and all packages pass。

- [ ] **Step 2: Run frontend verification**

Run: `cd web && pnpm install --frozen-lockfile && pnpm build`
Expected: build and vue-tsc pass。

- [ ] **Step 3: Run isolated integration smoke test**

使用独立数据目录和 fake Provider/cfst 验证：真实 DNS 记录读取 → 当前 IP 精确检测 → 连续异常 → 冷却 → health-triggered full Run → DNS 更新 → 历史/API/UI 展示；至少覆盖 v4/v6、Top-N、线路、proxied skip、Provider unknown。

- [ ] **Step 4: Perform workspace diagnostics and review**

运行 `git diff --check`，检查四语言键一致、API 文档与实现一致、旧任务/旧备份兼容、健康历史不会进入普通运行统计。确认不修改用户已有的非本功能文件。

- [ ] **Step 5: Update project progress**

在 `docs/PLAN.md` 增加 Issue #18 阶段、验证命令和已知限制；不手动修改 CHANGELOG 或 release-please 文件。

- [ ] **Step 6: Commit the completed feature**

在功能分支上按仓库约定提交，例如：

```bash
git add internal web docs
 git commit -m "feat: 增加优选 IP 健康检测"
```

提交前确认没有把用户无关的工作区改动加入 commit。
