// 与 docs/API.md 一一对应的类型定义

// ---------- 通用 ----------
export type FieldType = 'text' | 'password' | 'number' | 'switch' | 'select' | 'textarea'

export interface FieldOption {
  label: string
  value: string
}

export interface Field {
  key: string
  label: string
  type: FieldType
  required?: boolean
  secret?: boolean
  placeholder?: string
  help?: string
  default?: string
  options?: FieldOption[]
  showIf?: { key: string; value: string }
}

export interface TypeMeta {
  type: string
  name: string
  description?: string
  docsUrl?: string
  fields: Field[]
}

/** 所有配置值统一以字符串存储：switch 为 "true"/"false"，number 为数字字符串 */
export type Config = Record<string, string>

export interface OkResponse {
  ok: boolean
}

export interface TestResult {
  ok: boolean
  message: string
}

// ---------- 认证 ----------
export interface AuthStatus {
  initialized: boolean
}

export interface Credentials {
  username: string
  password: string
}

export interface LoginResult {
  token: string
  expiresAt: string
  username: string
}

export interface Me {
  username: string
}

/** 修改密码后旧令牌全部失效，响应携带新令牌 */
export interface ChangePasswordResult extends LoginResult {
  ok: boolean
}

export interface ChangePasswordRequest {
  oldPassword: string
  newPassword: string
}

// ---------- 系统 ----------
export interface SystemInfo {
  version: string
  commit: string
  buildTime: string
  /** CI 构建号（run_number），本地构建为空 */
  buildId: string
  /** CI 运行 ID，用于拼接 Actions 运行页链接 */
  runId: string
  repoUrl: string
  goVersion: string
  os: string
  arch: string
  dataDir: string
  startedAt: string
  timezone: string
}

export type UpdateChannel = 'release' | 'nightly' | 'dev'

export interface UpdateInfo {
  channel: UpdateChannel
  current: string
  latest?: string
  available: boolean
  url?: string
  publishedAt?: string
  /** 未检查过时为空 */
  checkedAt?: string
  error?: string
}

export interface LogLine {
  time: string
  level: string
  message: string
}

export interface SystemLogs {
  lines: LogLine[]
}

// ---------- DNS 账号 ----------
export interface Account {
  id: number
  name: string
  provider: string
  config: Config
  remark: string
  createdAt: string
  updatedAt: string
  taskCount: number
}

export interface AccountCreate {
  name: string
  provider: string
  config: Config
  remark: string
}

export interface AccountUpdate {
  name: string
  config: Config
  remark: string
}

export interface DNSRecord {
  id: string
  /** 主机记录 rr，如 www、@ */
  name: string
  fqdn: string
  type: string
  value: string
  ttl: number
  proxied?: boolean
  line?: string
}

// ---------- 通知渠道 ----------
export interface Notifier {
  id: number
  name: string
  type: string
  enabled: boolean
  config: Config
  onSuccess: boolean
  onFailure: boolean
  onlyOnChange: boolean
  /** 接收新版本提醒，与任务绑定无关 */
  onUpdate: boolean
  createdAt: string
  updatedAt: string
}

export type NotifierInput = Omit<Notifier, 'id' | 'createdAt' | 'updatedAt'>

// ---------- 任务 ----------
export type IPType = 'v4' | 'v6' | 'both'

export interface SpeedTestConfig {
  threads: number
  pingTimes: number
  downloadCount: number
  downloadTime: number
  port: number
  url: string
  httping: boolean
  httpingCode: number
  cfColo: string
  maxLatency: number
  minLatency: number
  maxLossRate: number
  minSpeed: number
  disableDownload: boolean
  allIP: boolean
  ipSource: 'default' | 'custom'
  ipv4Ranges: string
  ipv6Ranges: string
  extraArgs: string
  /** 测速网络：direct 要求直连，检测到代理时不更新 DNS；proxy 为有意经透明代理测速，强制 HTTPing */
  networkMode: NetworkMode
}

export type NetworkMode = 'direct' | 'proxy'


export interface UpdatePolicy {
  recordCount: number
  skipUnchanged: boolean
}

export interface Target {
  accountId: number
  domain: string
  rr: string
  ttl: number
  proxied: boolean
  line: string
}

export interface Task {
  id: number
  name: string
  enabled: boolean
  /** 空字符串 = 仅手动 */
  cron: string
  ipType: IPType
  speedTest: SpeedTestConfig
  update: UpdatePolicy
  targets: Target[]
  notifierIds: number[]
  createdAt: string
  updatedAt: string
  // 只读
  running: boolean
  nextRunAt: string | null
  lastRun: RunSummary | null
}

export type TaskInput = Pick<
  Task,
  'name' | 'enabled' | 'cron' | 'ipType' | 'speedTest' | 'update' | 'targets' | 'notifierIds'
>

export interface RunStarted {
  runId: number
}

export interface CronPreview {
  next: string[]
}

// ---------- 执行记录 ----------
export type RunStatus = 'queued' | 'running' | 'success' | 'partial' | 'failed' | 'canceled'
export type RunTrigger = 'manual' | 'cron' | 'hook'

export interface RunSummary {
  id: number
  taskId: number
  taskName: string
  trigger: RunTrigger
  status: RunStatus
  startedAt: string | null
  finishedAt: string | null
  durationMs: number
  bestIPv4: string
  bestIPv6: string
  bestLatency: number
  bestSpeed: number
  changed: boolean
  /** 试运行：只测速，不修改 DNS、不发通知 */
  dryRun: boolean
  message: string
  /** 固定说明的代码，前端据此按语言显示；上游报错等无代码时直接显示 message */
  messageKey?: string
  messageArgs?: Record<string, unknown>
  createdAt: string
}

export interface SpeedResult {
  ipType: 'v4' | 'v6'
  rank: number
  ip: string
  sent: number
  received: number
  lossRate: number
  /** ms */
  latency: number
  /** MB/s */
  speed: number
  colo: string
}

export type DNSChangeAction = 'create' | 'update' | 'delete' | 'skip' | 'error'

export interface DNSChange {
  accountId: number
  accountName: string
  fqdn: string
  type: 'A' | 'AAAA'
  action: DNSChangeAction
  oldValue: string
  newValue: string
  message: string
  /** 固定说明的代码，前端据此按语言显示；上游报错等无代码时直接显示 message */
  messageKey?: string
  messageArgs?: Record<string, unknown>
}

/** 测速前对单一 IP 类型的网络自检 */
export interface NetworkProbe {
  ipType: 'v4' | 'v6'
  /** 公网不可达的保留地址也能握手成功：连接被本地代理 / TUN / 网关拦截 */
  intercepted: boolean
  /** 到 Cloudflare IP 的 TCP 握手耗时，0 表示未连通 */
  connectMs: number
  /** Cloudflare 看到的出口 */
  egressIP: string
  loc: string
  colo: string
  /** 国内回显服务看到的出口 */
  localIP: string
  error?: string
}

export interface NetworkCheck {
  mode: NetworkMode
  probes: NetworkProbe[]
  proxySuspected: boolean
  reason?: 'intercepted' | 'latency'
}

export interface RunDetail extends RunSummary {
  results: SpeedResult[]
  /** 各 IP 类型实际测得的结果数；results 只保存前 100 个（旧记录无此字段） */
  resultTotals?: Partial<Record<'v4' | 'v6', number>>
  changes: DNSChange[]
  log: string
  /** 网络自检（旧记录无此字段） */
  network?: NetworkCheck
}

export interface RunListQuery {
  taskId?: number
  status?: RunStatus
  page?: number
  size?: number
}

export interface RunList {
  items: RunSummary[]
  total: number
}

export interface DeletedCount {
  deleted: number
}

// ---------- 仪表盘 ----------
export interface DashboardStats {
  taskCount: number
  enabledTaskCount: number
  accountCount: number
  notifierCount: number
  runs24h: number
  success24h: number
  failed24h: number
}

export interface DashboardRecord {
  taskId: number
  taskName: string
  fqdn: string
  type: string
  value: string
  updatedAt: string
}

export interface TrendPoint {
  time: string
  taskId: number
  taskName: string
  ipType: 'v4' | 'v6'
  latency: number
  speed: number
}

export interface UpcomingRun {
  taskId: number
  taskName: string
  nextRunAt: string
}

export interface Dashboard {
  stats: DashboardStats
  cfst: { installed: boolean; version: string }
  active: RunSummary[]
  lastRuns: RunSummary[]
  records: DashboardRecord[]
  trend: TrendPoint[]
  upcoming: UpcomingRun[]
}

// ---------- cfst ----------
export interface CfstStatus {
  installed: boolean
  version: string
  path: string
  os: string
  arch: string
  asset: string
  installing: boolean
}

export interface CfstRelease {
  tag: string
  name: string
  publishedAt: string
  assetAvailable: boolean
}

// cfst 导入 / 镜像测速相关类型见 types-p9.ts

export type IPFileKind = 'v4' | 'v6'

export interface IPFile {
  content: string
  lines: number
}

// ---------- 设置 ----------
export interface Settings {
  githubMirror: string
  historyRetentionDays: number
  hookEnabled: boolean
  hookToken: string
  notifyTitlePrefix: string
  /** 仪表盘入门清单已被关闭 */
  onboardingDismissed: boolean
  /** 自动检查新版本 */
  updateCheck: boolean
}
