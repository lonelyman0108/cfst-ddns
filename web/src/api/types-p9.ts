// P9 新增类型（引导、cfst 导入、镜像测速、v1 配置导入），与 .omc/plans/p9-contract.md 对应

// ---------- cfst 导入 / 识别 ----------
export interface CfstCandidate {
  path: string
  source: 'datadir' | 'path' | 'bundled'
  version: string
  os: string
  arch: string
  compatible: boolean
  message?: string
}

export interface CfstImportResult {
  version: string
  os: string
  arch: string
}

// ---------- GitHub 镜像 ----------
export interface MirrorPreset {
  mirror: string
  label: string
}

export interface MirrorProbe extends MirrorPreset {
  ok: boolean
  latencyMs: number
  error?: string
}

// ---------- v1 配置导入 ----------
export interface LegacyAccount {
  name: string
  provider: string
  config: Record<string, string>
}

export interface LegacyNotifier {
  name: string
  type: string
  config: Record<string, string>
  onSuccess: boolean
  onFailure: boolean
  onChangeOnly: boolean
}

export interface LegacyTask {
  name: string
  cron: string
  ipType: string
  summary: string
}

export interface LegacyPreview {
  accounts: LegacyAccount[]
  notifiers: LegacyNotifier[]
  tasks: LegacyTask[]
  settings: { githubMirror?: string }
  warnings: string[]
}

export interface LegacyApplyResult {
  created: { accounts: number; notifiers: number; tasks: number }
  warnings: string[]
}

// ---------- 入门清单 ----------
export type OnboardingKey = 'cfst' | 'account' | 'notifier' | 'task' | 'run'

export interface OnboardingState {
  cfst: boolean
  account: boolean
  notifier: boolean
  task: boolean
  run: boolean
}
