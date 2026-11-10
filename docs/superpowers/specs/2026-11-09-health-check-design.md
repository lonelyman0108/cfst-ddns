# Issue #18：优选 IP 健康检测与自动切换设计

- 日期：2026-11-09
- 状态：待用户审批
- 关联 Issue：https://github.com/lonelyman0108/cfst-ddns/issues/18

## 1. 意图

任务当前通过 cron 定期执行完整 CloudflareSpeedTest，并将结果写入 DNS。Issue #18 希望在两次完整测速之间增加轻量健康检查：定时读取 DNS 服务商当前实际记录中的 IP，只测试这些 IP 的延迟/速度；连续异常达到阈值后，才触发一次完整测速，找到合格的新 IP 后更新 DNS。

健康检查的 IP 来源已确定为：**每次从 DNS 服务商读取实际记录**，不以 `RecordState` 作为唯一来源。`RecordState` 仍用于仪表盘和最近写入状态展示。

## 2. 目标

- 每个任务可单独开启健康检查，并配置检查间隔。
- 支持延迟上限、最低速度两个独立阈值，至少配置一个。
- 对任务所有 Target 的 A/AAAA 当前记录去重后检测；Top-N 的每个当前 IP 都参与检测。
- 单次异常不立即切换；达到连续异常次数后触发完整选优。
- 支持冷却时间，避免健康检查重复触发完整测速。
- 真实 DNS 查询失败时记录为未知，不因凭据、限流或上游暂时不可用而自动改 DNS。
- 保存每次检测的 IP、延迟、速度、丢包率、判断结果和原因。
- 只有完整测速成功且 DNS 同步产生了合适结果时，才算完成切换。
- 健康检查与完整测速共用全局串行执行队列，避免争抢带宽。

## 3. 非目标与反目标

- 不把健康检查改造成每次都扫描完整 IP 段。
- 不使用系统 DNS 解析结果代替 DNS 服务商 API 返回的记录；本功能检测的是任务实际维护的记录值。
- 不为每次健康检查创建普通 `Run`，避免历史列表和 24 小时统计被高频检查刷满。
- 不在单次健康异常时发送通知；仅在健康检查触发完整任务且已有任务通知策略适用时复用现有通知。
- 第一版不新增独立的用户自定义 HTTP 探测器；速度/延迟测量继续复用 CloudflareSpeedTest。
- Cloudflare `proxied=true` 的记录不作为客户端实际访问该 IP 的可靠依据；**用户已确认第一版跳过这类 Target，并在界面和健康历史中提示**，而不是据此触发切换。同一任务中的非代理 Target 仍正常检测。

## 4. 方案比较与选定方案

### 方案 A：定时完整测速

每次间隔直接执行现有任务。改动最小，但开销大、没有当前 IP 退化判断，也无法自然实现连续异常和冷却，因此不采用。

### 方案 B：Go 原生 TCP/HTTP 探测

延迟可以轻量实现，但速度测试需要自行处理下载 URL、Host/SNI、HTTPing、IPv4/IPv6 和代理模式，结果也会偏离现有 cfst。只适合未来的纯延迟探测，不作为本功能主路径。

### 方案 C：cfst 指定当前 IP（选定）

CloudflareSpeedTest 支持 `-ip 1.1.1.1,2.2.2.2`，也支持文件中的单 IP。扩展现有 cfst runner，使健康检查只将当前 DNS IP 传给 cfst；健康异常后再通过现有 Engine 执行完整测速和 DNS 同步。这样延迟、速度、HTTPing、代理环境清理和结果解析均沿用现有实现。

## 5. 配置模型

在 `store.Task` 中增加序列化字段：

```go
type HealthCheckConfig struct {
    Enabled             bool    `json:"enabled"`
    IntervalMinutes     int     `json:"intervalMinutes"`
    MaxLatency          float64 `json:"maxLatency"`       // 0 = 不限制，单位 ms
    MinSpeed            float64 `json:"minSpeed"`         // 0 = 不限制，单位 MB/s
    ConsecutiveFailures int     `json:"consecutiveFailures"`
    CooldownMinutes     int     `json:"cooldownMinutes"`
}
```

默认值建议：

- `Enabled=false`
- `IntervalMinutes=10`
- `MaxLatency=0`
- `MinSpeed=0`
- `ConsecutiveFailures=3`
- `CooldownMinutes=30`

校验规则：

- 间隔至少 1 分钟；上限由实现设置为 24 小时。
- `MaxLatency`、`MinSpeed` 至少一个大于 0。
- 连续异常次数至少 1；建议上限 20。
- 冷却时间为 0 表示不冷却；否则至少 1 分钟，建议上限 7 天。
- 任务没有可检测的非 proxied Target 时，保存配置可以成功，但界面显示健康检查无可用目标，不启动有效检测。

健康阈值独立于 `SpeedTestConfig.maxLatency/minSpeed`：后者是完整测速筛选候选 IP 的参数，前者是判断现有 DNS IP 是否退化的参数。

## 6. 持久化模型

### 6.1 `HealthState`

每个任务一行，保存触发状态，不把运行时状态塞进 Task 配置：

```go
type HealthState struct {
    TaskID              uint       `gorm:"primaryKey" json:"taskId"`
    ConsecutiveFailures int        `json:"consecutiveFailures"`
    LastCheckedAt       *time.Time `json:"lastCheckedAt"`
    LastHealthy         *bool      `json:"lastHealthy"`
    CooldownUntil       *time.Time `json:"cooldownUntil"`
    LastTriggeredRunID  *uint      `json:"lastTriggeredRunId"`
    UpdatedAt           time.Time  `json:"updatedAt"`
}
```

### 6.2 `HealthCheck`

每次健康检查一行，结果数组使用 GORM JSON serializer：

```go
type HealthResult struct {
    TargetKey string  `json:"targetKey"` // account/fqdn/type/line 的稳定组合
    IPType    string  `json:"ipType"`
    IP        string  `json:"ip"`
    Latency   float64 `json:"latency"`
    Speed     float64 `json:"speed"`
    LossRate  float64 `json:"lossRate"`
    Healthy   bool    `json:"healthy"`
    Reason    string  `json:"reason,omitempty"`
}

type HealthCheck struct {
    ID             uint           `gorm:"primaryKey" json:"id"`
    TaskID         uint           `gorm:"index" json:"taskId"`
    CheckedAt      time.Time      `gorm:"index" json:"checkedAt"`
    Status         string         `json:"status"` // healthy/unhealthy/unknown/skipped
    FailureReason  string         `json:"failureReason,omitempty"`
    Results        []HealthResult `gorm:"serializer:json" json:"results"`
    TriggeredRunID *uint          `json:"triggeredRunId,omitempty"`
    CreatedAt      time.Time      `json:"createdAt"`
}
```

数据库通过现有 `AutoMigrate` 增加表。健康历史跟随普通历史保留策略清理；清理时应同时删除过期 `HealthCheck`，不能只清理 `Run`。

## 7. 读取真实 DNS 当前 IP

健康任务执行时，对每个 Target 和任务 IP 类型执行：

1. 读取账号并构造 Provider。
2. 调用 `Provider.ListRecords(domain, rr, type)`。
3. 使用现有线路匹配逻辑，只保留目标线路的记录。
4. 过滤 `proxied=true` 的不可检测 Target，并记录跳过原因。
5. 将实际记录值按 `TargetKey + IPType + IP` 去重，保留 IP 到 Target 的映射供结果展示。
6. 对每种 IP 类型形成 cfst 的精确 IP 列表。

Provider 查询错误属于 `unknown`：保存错误信息、不中断其他 Target 的记录，但不增加连续异常次数，也不触发 DNS 切换。查询成功但没有可检测记录时记录为 `skipped`；是否将“任务所有记录均缺失”视为异常由实现统一处理为可恢复异常，达到阈值后允许完整任务补建记录。

健康检查应设置每个 Provider 查询的独立超时，并复用现有 Provider 的线路语义。不能使用 `RecordState` 回退到旧 IP 后继续检测，否则会违背“以实际 DNS 记录为准”。

## 8. cfst 健康测速

扩展 `internal/cfst/runner.go`：

- `RunOptions` 增加可选的精确 IP 列表，或新增专用的 `RunIPs` 方法。
- 精确列表模式生成 `-ip <逗号分隔 IP>`，不使用默认 IP 段文件。
- 仍使用 `-o` 输出临时 CSV、`-p 0` 禁止终端结果，复用现有 CSV 解析和 `withoutProxyEnv`。
- 健康检测需要拿到原始指标，因此不要用 cfst 的 `-tl`、`-sl` 过滤掉不合格 IP；阈值在 Go 中判断。
- 只配置延迟阈值时强制 `-dd`，避免不必要的下载。
- 配置最低速度时保留下载测试，并将下载测试数量设置为当前 IP 数量，确保每个当前 IP 都获得速度结果。
- 任务的端口、URL、HTTPing、HTTPing 状态码、Cloudflare colo 和代理网络模式沿用现有测速配置。
- 精确 IP 列表为空时不启动 cfst，直接记录 `skipped`。

健康结果判断：

```text
healthy =
  (maxLatency == 0 || latency <= maxLatency) &&
  (minSpeed == 0 || speed >= minSpeed)
```

缺少 cfst 结果、超时、完全丢包或无法连通的当前 IP 视为该 IP unhealthy；Provider 查询失败仍是 unknown，不计入连续异常。

直连模式健康检查应复用现有 `netcheck` 的代理检测，发现保留地址被拦截或明显的本地握手异常时记录 `unknown/proxyDetected`，不触发重新测速。经代理模式继续强制 HTTPing，和正式任务保持一致。

## 9. 调度与执行队列

### 9.1 Scheduler

`internal/scheduler.Scheduler` 为每个任务维护两类 entry：

- 现有 cron entry：触发完整任务。
- health entry：当 `HealthCheck.Enabled` 时注册 `@every <IntervalMinutes>m`。

任务新增、编辑、启停和删除时同时同步两类 entry。任务禁用时两类 entry 都移除。任务 API 的只读视图增加健康检查下次执行时间，避免把健康检查时间误显示为完整任务的 `nextRunAt`。

### 9.2 Engine 队列

当前队列是 `chan uint`，只表示普通 Run。改为携带类型的 job：

```go
type job struct {
    kind   string // full / health
    taskID uint
    runID  uint
}
```

健康 job 和完整测速共用一个 worker，确保不会并发启动 cfst。健康 job 不创建普通 Run；它完成后如果达到触发条件，再通过现有 `Enqueue(taskID, "health", false)` 创建完整 Run。

健康 job 需要有任务级去重状态，避免同一任务出现多个待处理健康 job。执行健康 job 时若任务已有普通 Run 排队或运行，记录 `skipped`，不增加失败次数。

`Run.Trigger` 增加 `health`，前端的 `RunTrigger` 和 i18n 增加对应展示。健康触发的完整任务仍使用现有 DNS 同步、通知、SSE 和取消机制。

## 10. 连续异常、冷却与状态转移

每次健康检查结束后：

- `healthy`：连续异常归零，更新最后检查时间。
- `unhealthy`：连续异常加一；未达到阈值只保存记录，不切换。
- 达到阈值且当前不在冷却期：触发完整任务，写入 `LastTriggeredRunID`，设置 `CooldownUntil`。
- 达到阈值但仍在冷却期：继续保存 unhealthy，不重复触发。
- `unknown` / `skipped`：只更新时间和历史，不改变连续异常计数。

完整任务只有在 DNS 同步实际成功并产生可用目标结果后，才清除健康异常状态。完整测速失败、没有合格 IP 或 DNS 更新失败时，不清除异常状态，但冷却期仍然生效，避免故障期间持续启动全量测速。

任务被手动或 cron 完整执行并成功更新 DNS 时，也清除健康异常计数，因为当前 IP 已经被替换；完整执行成功但 DNS IP 未变化时不强行清除仍存在的健康异常。

## 11. API 与界面

### API

任务读写接口的 Task 数据增加：

```ts
healthCheck: HealthCheckConfig
healthStatus: {
  lastCheckedAt: string | null
  lastHealthy: boolean | null
  consecutiveFailures: number
  cooldownUntil: string | null
  lastTriggeredRunId: number | null
}
healthNextRunAt: string | null
```

新增：

```text
GET /api/tasks/:id/health?limit=50
```

响应包含当前 `healthStatus` 和最近健康检查列表。限制最大 200，按时间倒序。

`GET /api/runs` 的 `trigger` 支持 `health`。现有 `RunDetail` 中的 DNS 变更继续展示切换前后 IP；健康检查详情通过任务健康历史接口查看。

备份恢复应包含任务中的健康配置，并包含健康状态/历史以便迁移后不丢失冷却状态；恢复时按现有事务和类型校验处理旧备份缺少字段的情况。

### 前端

- `TaskEditView` 增加“健康检查”区块：启用开关、间隔、最大延迟、最低速度、连续异常次数、冷却时间。
- 说明文字直接显示在字段下方，不使用问号 tooltip。
- 任务列表或任务详情显示最近检查状态、连续异常次数和冷却剩余时间。
- 任务详情增加最近健康检查记录，可看到每个实际 DNS IP 的延迟、速度、状态和原因。
- Run 列表/详情将 `health` 作为“健康检查触发”显示，并继续复用现有 DNS 变更展示。
- 新增文案同步加入 `zh-CN`、`zh-TW`、`en`、`ja` 四套 locale。
- 遵守现有窄屏、超长 IP、暗色模式和表格不横向溢出约定。

## 12. 验证计划

### 后端单测

- 健康配置默认值、校验和旧数据库兼容。
- 精确 IP 模式的 cfst 参数构造：IPv4、IPv6、多 IP、代理模式、只延迟、带速度。
- Provider 返回多目标、多线路、重复 IP、proxied 记录时的收集和去重。
- 延迟/速度阈值组合判断。
- healthy/unhealthy/unknown/skipped 状态转移。
- 连续异常阈值、冷却期、触发一次性和完整任务成功/失败后的重置。
- 健康 job 与完整 Run 的队列串行和任务去重。
- 健康触发完整 Run 的 `trigger=health`、DNS 变更和通知行为。
- 健康历史清理、备份恢复。

### 集成验证

- 使用 fake Provider 返回当前 A/AAAA 记录。
- 使用 fake cfst 验证只测试当前 IP，不扫描默认 IP 文件。
- 模拟前两次异常、第三次触发、冷却期不重复触发、完整任务成功更新。
- 模拟 DNS API 错误，确认只记 unknown，不改 DNS。
- `go vet ./... && go test ./...`。
- `cd web && pnpm install --frozen-lockfile && pnpm build`。
- 隔离实例验证任务编辑、健康历史、健康触发 Run，以及 1440/1024/768/390 四种宽度下无溢出。

## 13. 留给实现者的细节

- 可将 `HealthResult.TargetKey` 的具体编码、健康历史清理的 SQL、Scheduler entry map 的具体拆分方式作为实现细节决定。
- 可根据现有备份结构选择显式增加健康表字段或由通用模型备份逻辑自动覆盖，但必须保证旧备份可恢复。
- 若某个 Provider 不支持读取带线路的记录，应沿用其现有 `ListRecords` 语义并将无法判定的情况记录为 unknown，不能猜测线路。
