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
  lastRun: RunSummary | null   // 最近一次正式执行（不含试运行）
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
| POST | /api/tasks/:id/run | 可选 `{dryRun?: boolean}`（请求体可省略） | `{runId}`；已在运行或排队时 409（试运行同样去重） |
| GET | /api/cron/preview?expr=0 */6 * * * | — | `{next: string[5]}`；表达式非法 400 |

试运行（`dryRun: true`）：只测速并记录结果，不修改 DNS、不发送通知、不写入仪表盘「当前记录」、不计入趋势与 24 小时统计、不作为任务的 `lastRun`；日志中打印「试运行：跳过 DNS 同步与通知」，成功时 message 为「试运行完成，未修改 DNS」。试运行不要求任务配置目标记录。定时与 Webhook 触发始终为正常执行。

## 执行记录

```ts
type RunStatus = 'queued' | 'running' | 'success' | 'partial' | 'failed' | 'canceled'
interface RunSummary { id: number; taskId: number; taskName: string; trigger: 'manual'|'cron'|'hook'
  dryRun: boolean  // 试运行
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
  stats: { taskCount, enabledTaskCount, accountCount, notifierCount, runs24h, success24h, failed24h }  // 24h 统计不含试运行
  cfst: { installed: boolean; version: string }
  active: RunSummary[]
  lastRuns: RunSummary[]                // 最近 10 次
  records: { taskId, taskName, fqdn, type, value, updatedAt }[]   // 每个目标记录最近一次写入的值
  trend: { time, taskId, taskName, ipType, latency, speed }[]     // 最近 50 次成功执行的最优值（不含试运行）
  upcoming: { taskId, taskName, nextRunAt }[]
}
```

## cfst 管理

| 方法 | 路径 | 请求 | 响应 |
|---|---|---|---|
| GET | /api/cfst | — | `{installed, version, path, os, arch, asset, installing}`（服务首次启动时会自动安装，期间 `installing=true`） |
| GET | /api/cfst/releases | — | `{tag, name, publishedAt, assetAvailable}[]` |
| POST | /api/cfst/install | `{version: 'latest' \| 'v2.3.5'}` | `{version}`（同步，最长约 3 分钟）；正在安装或有任务排队 / 运行时 409 |
| GET | /api/cfst/ipfile/:kind | kind=v4\|v6 | `{content, lines}` |
| PUT | /api/cfst/ipfile/:kind | `{content}` | `{ok}` |
| POST | /api/cfst/ipfile/:kind/reset | — | `{content, lines}` |
| POST | /api/cfst/upload | `multipart/form-data`：`file`（zip / tar.gz / 裸可执行文件，≤ 64MB），可选 `version` | `CfstImportResult`；平台不匹配、无法执行或格式错误 400（如「文件为 linux/amd64，本机为 darwin/arm64」）；超过 64MB 413；正在安装或有任务排队 / 运行时 409 |
| GET | /api/cfst/scan | — | `CfstCandidate[]`（不含已登记的安装本身及内容相同的文件；逐个试运行，约数秒） |
| POST | /api/cfst/adopt | `{path, version?}`（path 须为 scan 返回的路径） | `CfstImportResult`（复制到数据目录并写 VERSION）；错误码同 upload |
| GET | /api/cfst/mirrors | — | `{mirror, label}[]` 内置镜像预设，第一项为 `{mirror: '', label: '直连 GitHub'}` |
| POST | /api/cfst/mirrors/test | 可选 `{mirrors?: string[]}`（省略时测全部预设 + 当前设置值；最多 20 个） | `MirrorProbe[]`，按可用、耗时排序 |

```ts
interface CfstImportResult { version: string; os: string; arch: string }
interface CfstCandidate {
  path: string                               // 绝对路径（已解析符号链接）
  source: 'datadir' | 'path' | 'bundled'     // 数据目录未登记版本（含数据目录根下的 cfst / CloudflareST）/ PATH / 预置目录
  version: string                            // 未知为 ''
  os: string; arch: string                   // 从文件头解析，未知为 ''
  compatible: boolean                        // 与本机平台匹配且能执行
  message?: string                           // 不兼容原因
}
interface MirrorProbe { mirror: string; label: string; ok: boolean; latencyMs: number; error?: string }
```

- 平台识别：解析 ELF / Mach-O / PE 文件头；ARM 不区分 GOARM 版本。除同名架构外，amd64 主机允许 386、arm64 主机允许 arm 与 amd64（Rosetta），能否运行以试运行结果为准。
- 版本识别顺序：请求中的 `version`（可省略 `v` 前缀）→ 文件名 / 压缩包名中的 `vX.Y.Z` → 在临时目录执行 `<bin> -v`（5 秒超时）取第一个 `vX.Y.Z` → `unknown`。
- 导入裸二进制时若安装目录还没有 `ip.txt` / `ipv6.txt`，写入内置的 Cloudflare 官方 IP 段。
- 镜像测速：并发请求一个固定版本的发布文件（`Range: bytes=0-1023`），单个 8 秒超时；返回非 200/206、HTML 页面或非 gzip 内容均视为失败。预设使用 `{url}` 写法（`https://ghfast.top/{url}` 等），可直接写入 `settings.githubMirror`。

## 设置 / 备份 / Webhook

```ts
interface Settings { githubMirror: string; historyRetentionDays: number; hookEnabled: boolean; hookToken: string; notifyTitlePrefix: string
  onboardingDismissed: boolean  // 仪表盘入门清单已关闭，默认 false；随备份导出与恢复
}
```

| 方法 | 路径 | 请求 | 响应 |
|---|---|---|---|
| GET | /api/settings | — | `Settings` |
| PUT | /api/settings | `Partial<Settings>`（hookToken 忽略） | `Settings` |
| POST | /api/settings/hook-token | — | `Settings`（重新生成） |
| GET | /api/backup | — | 下载 JSON 文件（含明文凭据） |
| POST | /api/backup/restore | 备份 JSON | `{ok}`（覆盖账号/通知/任务/设置） |
| GET/POST | /api/hooks/tasks/:id/run?token= | 无需登录，需 hookEnabled | `{runId}`（始终为正常执行，不支持试运行） |

初始化时恢复备份不需要新接口：先 `POST /api/auth/setup` 创建管理员，拿到 token 后再调用 `POST /api/backup/restore`。

## 旧版（v1）配置导入

| 方法 | 路径 | 请求 | 响应 |
|---|---|---|---|
| POST | /api/import/legacy | `{content: string, apply: boolean}` | `apply=false` → `LegacyPreview`；`apply=true` → `{created: {accounts, notifiers, tasks}: number, warnings: string[]}`；content 为空 400 |

```ts
interface LegacyPreview {
  accounts:  { name: string; provider: string; config: Config }[]   // 密钥字段以 '******' 掩码
  notifiers: { name: string; type: string; config: Config; onSuccess: boolean; onFailure: boolean; onChangeOnly: boolean }[]
  tasks:     { name: string; cron: string; ipType: string; summary: string }[]
  settings:  { githubMirror?: string }
  warnings:  string[]
}
```

- content 可为 v1 的 `config.sh`、`.env` 或 docker-compose `environment:` 片段，支持 `KEY=VALUE`、`export KEY=...`、`- KEY=VALUE`、`- "KEY=VALUE"`、`KEY: VALUE`、引号、`#` 注释与 `${VAR:-默认值}`；值为纯引用 `${VAR}` / `$VAR` 时按空值处理并进 warnings（单引号内按字面值）；只识别全大写变量名，同名变量以最后一次为准。
- 映射：`DNS_PROVIDER`（缺省按凭据推断，默认 cloudflare）+ `CF_API_TOKEN` / `CF_EMAIL` + `CF_API_KEY` / `CF_ZONE_ID`（示例占位值忽略）或 `DNSPOD_TOKEN`（`ID,Token`）→ 一个账号；`ENABLE_BARK` + `BARK_URL` / `BARK_KEY`（密钥也可写在 `BARK_URL` 末尾）、`ENABLE_TELEGRAM` + `TG_BOT_TOKEN` / `TG_CHAT_ID` → 通知渠道（成功、失败都通知）；`CFST_TEST_MODE` → IP 类型；`CRON_SCHEDULE` → 执行周期（`ENABLE_CRON=false` 为仅手动，未配置时用 `0 */6 * * *` 并提示）；`CFST_PARAMS` → 结构化测速参数（`-ip` 转为自定义 IP 段，`-o` / `-p` 忽略，`-f` 提示手动处理，其余未知参数进附加参数）；`GITHUB_MIRROR` → `settings.githubMirror`。未识别或已移除的变量（`SKIP_SPEED_TEST`、`CFST_VERSION` 等）进 warnings。
- 任务只在账号解析成功时创建，并关联导入的通知渠道。`apply=true` 时用账号的域名列表把 `DNS_RECORD_NAMES` 拆成主域名 + 主机记录（取最长匹配，TTL：Cloudflare 为 1 自动，其他 600）；获取域名列表失败或找不到所属域名的记录不会猜测，进 warnings。一条目标都没有时任务仍会创建但为停用状态，需编辑补充后启用。网络请求完成后，账号、渠道、任务与设置在同一个事务中写入，失败时数据不改动（500）。

