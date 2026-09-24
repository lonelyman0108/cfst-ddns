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
  goVersion: string
  os: string
  arch: string
  dataDir: string
  startedAt: string
  timezone: string
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
}

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
  message: string
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
}

export interface RunDetail extends RunSummary {
  results: SpeedResult[]
  changes: DNSChange[]
  log: string
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
}
