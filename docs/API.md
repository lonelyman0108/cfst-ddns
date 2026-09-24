# cfst-ddns v2 API 约定

- 前缀 `/api`，JSON，时间为 RFC3339 字符串。
- 鉴权：`Authorization: Bearer <token>`；SSE（EventSource 无法设置请求头）使用查询参数 `?token=<token>`。
- 成功：2xx，响应体直接为数据；无数据时返回 `{"ok": true}`。
- 失败：4xx/5xx，响应体 `{"error": "中文错误信息"}`。401 表示未登录或令牌失效。
- 密钥字段（Schema 中 `secret: true`）读取时返回 `"******"`；更新时传 `"******"` 或省略表示保持原值。

## 通用类型

```ts
type FieldType = 'text' | 'password' | 'number' | 'switch' | 'select' | 'textarea'
interface Field {
  key: string; label: string; type: FieldType
  required?: boolean; secret?: boolean
  placeholder?: string; help?: string; default?: string
  options?: { label: string; value: string }[]   // select
  showIf?: { key: string; value: string }        // 仅当另一字段等于该值时显示
}
interface TypeMeta { type: string; name: string; description?: string; docsUrl?: string; fields: Field[] }
// 所有配置值统一以字符串存储：switch 为 "true"/"false"，number 为数字字符串
type Config = Record<string, string>
```

## 认证

| 方法 | 路径 | 请求 | 响应 |
|---|---|---|---|
| GET | /api/auth/status | — | `{initialized: boolean}`（无需登录） |
| POST | /api/auth/setup | `{username, password}` | `{token, expiresAt, username}`（仅未初始化时可用） |
| POST | /api/auth/login | `{username, password}` | `{token, expiresAt, username}` |
| GET | /api/auth/me | — | `{username}` |
| POST | /api/auth/password | `{oldPassword, newPassword}` | `{ok, token, expiresAt, username}`（旧令牌全部失效，前端需替换为新 token） |

## 元数据 / 系统

| 方法 | 路径 | 响应 |
|---|---|---|
| GET | /api/meta/providers | `TypeMeta[]` |
| GET | /api/meta/notifiers | `TypeMeta[]` |
| GET | /api/system/info | `{version, commit, buildTime, goVersion, os, arch, dataDir, startedAt, timezone}` |
| GET | /api/system/logs?limit=500 | `{lines: {time, level, message}[]}` 应用日志（环形缓冲） |
| GET | /healthz | `ok`（无需登录） |

## DNS 账号

```ts
interface Account { id: number; name: string; provider: string; config: Config; remark: string; createdAt: string; updatedAt: string; taskCount: number }
interface DNSRecord { id: string; name: string /* 主机记录 rr，如 www、@ */; fqdn: string; type: string; value: string; ttl: number; proxied?: boolean; line?: string }
```

| 方法 | 路径 | 请求 | 响应 |
|---|---|---|---|
| GET | /api/accounts | — | `Account[]` |
| POST | /api/accounts | `{name, provider, config, remark}` | `Account` |
| PUT | /api/accounts/:id | `{name, config, remark}`（provider 不可改） | `Account` |
| DELETE | /api/accounts/:id | — | `{ok}`；被任务引用时 409 |
| POST | /api/accounts/test | `{provider, config, id?}`（未保存的配置；编辑已有账号时传 `id`，值为 `"******"` 的密钥字段用已保存值补齐） | `{ok: boolean, message: string}` |
| POST | /api/accounts/:id/test | — | `{ok: boolean, message: string}` |
| GET | /api/accounts/:id/domains | — | `string[]` |
| GET | /api/accounts/:id/records?domain=example.com&rr=www | — | `DNSRecord[]`（rr 可省略） |

## 通知渠道

```ts
interface Notifier { id: number; name: string; type: string; enabled: boolean; config: Config
  onSuccess: boolean; onFailure: boolean; onlyOnChange: boolean; createdAt: string; updatedAt: string }
```

| 方法 | 路径 | 请求 | 响应 |
|---|---|---|---|
| GET | /api/notifiers | — | `Notifier[]` |
| POST | /api/notifiers | `Omit<Notifier,'id'|'createdAt'|'updatedAt'>` | `Notifier` |
| PUT | /api/notifiers/:id | 同上 | `Notifier` |
| DELETE | /api/notifiers/:id | — | `{ok}` |
| POST | /api/notifiers/test | `{type, config, id?}`（同上，传 `id` 时补齐掩码密钥） | `{ok, message}` |
| POST | /api/notifiers/:id/test | — | `{ok, message}` |

## 任务

```ts
type IPType = 'v4' | 'v6' | 'both'
interface SpeedTestConfig {
  threads: number        // -n   默认 200
  pingTimes: number      // -t   默认 4
  downloadCount: number  // -dn  默认 10
  downloadTime: number   // -dt  默认 10 秒
  port: number           // -tp  默认 443
  url: string            // -url 默认 ''（使用 cfst 内置）
  httping: boolean       // -httping
  httpingCode: number    // -httping-code 0 表示默认
  cfColo: string         // -cfcolo 逗号分隔，仅 httping
  maxLatency: number     // -tl  默认 9999 ms
  minLatency: number     // -tll 默认 0
  maxLossRate: number    // -tlr 默认 1
  minSpeed: number       // -sl  默认 0 MB/s
  disableDownload: boolean // -dd
  allIP: boolean         // -allip
  ipSource: 'default' | 'custom'   // default 使用 cfst 目录下 ip.txt/ipv6.txt
  ipv4Ranges: string     // custom 时的 IPv4 段，换行或逗号分隔
  ipv6Ranges: string
  extraArgs: string      // 附加原始参数
}
interface UpdatePolicy {
  recordCount: number    // 每种类型写入前 N 个 IP（多条同名记录），默认 1
  skipUnchanged: boolean // IP 未变化时不调用更新接口，默认 true
}
interface Target {
  accountId: number
  domain: string         // 主域名 example.com
  rr: string             // 主机记录 www / @ / *.cdn
  ttl: number            // 默认 600；Cloudflare 1 = 自动
  proxied: boolean       // 仅 Cloudflare，默认 false
  line: string           // 线路（DNSPod/腾讯云/阿里云/华为云），空=默认
}
interface Task {
  id: number; name: string; enabled: boolean; cron: string  // 空字符串 = 仅手动
  ipType: IPType
  speedTest: SpeedTestConfig; update: UpdatePolicy
  targets: Target[]; notifierIds: number[]
  createdAt: string; updatedAt: string
  // 只读
  running: boolean; nextRunAt: string | null
  lastRun: RunSummary | null
}
```

| 方法 | 路径 | 请求 | 响应 |
|---|---|---|---|
| GET | /api/tasks | — | `Task[]` |
| GET | /api/tasks/defaults | — | 新建任务的默认值 `Task`（id=0） |
| GET | /api/tasks/:id | — | `Task` |
| POST | /api/tasks | Task 可写字段 | `Task` |
| PUT | /api/tasks/:id | Task 可写字段 | `Task` |
| PATCH | /api/tasks/:id/enabled | `{enabled}` | `Task` |
| DELETE | /api/tasks/:id | — | `{ok}` |
| POST | /api/tasks/:id/clone | — | `Task` |
| POST | /api/tasks/:id/run | — | `{runId}`；已在运行或排队时 409 |
| GET | /api/cron/preview?expr=0 */6 * * * | — | `{next: string[5]}`；表达式非法 400 |

## 执行记录

```ts
type RunStatus = 'queued' | 'running' | 'success' | 'partial' | 'failed' | 'canceled'
interface RunSummary { id: number; taskId: number; taskName: string; trigger: 'manual'|'cron'|'hook'
  status: RunStatus; startedAt: string | null; finishedAt: string | null; durationMs: number
  bestIPv4: string; bestIPv6: string; bestLatency: number; bestSpeed: number
  changed: boolean; message: string; createdAt: string }
interface SpeedResult { ipType: 'v4'|'v6'; rank: number; ip: string; sent: number; received: number
  lossRate: number; latency: number /* ms */; speed: number /* MB/s */; colo: string }
interface DNSChange { accountId: number; accountName: string; fqdn: string; type: 'A'|'AAAA'
  action: 'create'|'update'|'delete'|'skip'|'error'; oldValue: string; newValue: string; message: string }
interface RunDetail extends RunSummary { results: SpeedResult[]; changes: DNSChange[]; log: string }
```

| 方法 | 路径 | 响应 |
|---|---|---|
| GET | /api/runs?taskId=&status=&page=1&size=20 | `{items: RunSummary[], total}` |
| GET | /api/runs/active | `RunSummary[]`（queued + running） |
| GET | /api/runs/:id | `RunDetail` |
| POST | /api/runs/:id/cancel | `{ok}` |
| DELETE | /api/runs/:id | `{ok}` |
| DELETE | /api/runs?beforeDays=30 | `{deleted}` |
| GET | /api/runs/:id/stream?token= | SSE，见下 |

SSE 事件：
- `log`：`data` 为一行日志文本（连接时先补发已有日志）
- `progress`：`data` 为当前进度行（覆盖显示，不累积）
- `status`：`data` 为 `RunSummary` JSON
- `done`：`data` 为 `RunSummary` JSON，之后服务端关闭连接

## 仪表盘

`GET /api/dashboard` →

```ts
{
  stats: { taskCount, enabledTaskCount, accountCount, notifierCount, runs24h, success24h, failed24h }
  cfst: { installed: boolean; version: string }
  active: RunSummary[]
  lastRuns: RunSummary[]                // 最近 10 次
  records: { taskId, taskName, fqdn, type, value, updatedAt }[]   // 每个目标记录最近一次写入的值
  trend: { time, taskId, taskName, ipType, latency, speed }[]     // 最近 50 次成功执行的最优值
  upcoming: { taskId, taskName, nextRunAt }[]
}
```

## cfst 管理

| 方法 | 路径 | 请求 | 响应 |
|---|---|---|---|
| GET | /api/cfst | — | `{installed, version, path, os, arch, asset, installing}`（服务首次启动时会自动安装，期间 `installing=true`） |
| GET | /api/cfst/releases | — | `{tag, name, publishedAt, assetAvailable}[]` |
| POST | /api/cfst/install | `{version: 'latest' \| 'v2.3.5'}` | `{version}`（同步，最长约 3 分钟） |
| GET | /api/cfst/ipfile/:kind | kind=v4\|v6 | `{content, lines}` |
| PUT | /api/cfst/ipfile/:kind | `{content}` | `{ok}` |
| POST | /api/cfst/ipfile/:kind/reset | — | `{content, lines}` |

## 设置 / 备份 / Webhook

```ts
interface Settings { githubMirror: string; historyRetentionDays: number; hookEnabled: boolean; hookToken: string; notifyTitlePrefix: string }
```

| 方法 | 路径 | 请求 | 响应 |
|---|---|---|---|
| GET | /api/settings | — | `Settings` |
| PUT | /api/settings | `Partial<Settings>`（hookToken 忽略） | `Settings` |
| POST | /api/settings/hook-token | — | `Settings`（重新生成） |
| GET | /api/backup | — | 下载 JSON 文件（含明文凭据） |
| POST | /api/backup/restore | 备份 JSON | `{ok}`（覆盖账号/通知/任务/设置） |
| GET/POST | /api/hooks/tasks/:id/run?token= | 无需登录，需 hookEnabled | `{runId}` |
