import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { toast } from 'vue-sonner'
import type { DNSChangeAction, IPType, RunStatus, RunTrigger } from '@/api/types'
import { t } from '@/i18n'

dayjs.extend(relativeTime)

export { dayjs }

/** 格式化为本地时间 */
export function fmtTime(t?: string | null, fmt = 'YYYY-MM-DD HH:mm:ss'): string {
  if (!t) return '-'
  const d = dayjs(t)
  if (!d.isValid() || d.year() <= 1) return '-'
  return d.format(fmt)
}

/** 相对时间：3 分钟前 / 2 小时后 */
export function fromNow(t?: string | null): string {
  if (!t) return '-'
  const d = dayjs(t)
  if (!d.isValid() || d.year() <= 1) return '-'
  return d.fromNow()
}

export function fmtDuration(ms?: number | null): string {
  if (!ms || ms <= 0) return '-'
  const s = Math.round(ms / 1000)
  if (s < 60) return t('format.duration.s', { s })
  const m = Math.floor(s / 60)
  if (m < 60) return t('format.duration.ms', { m, s: s % 60 })
  return t('format.duration.hm', { h: Math.floor(m / 60), m: m % 60 })
}

export function fmtLatency(v?: number | null): string {
  if (v === undefined || v === null || v <= 0) return '-'
  return `${v.toFixed(2)} ms`
}

export function fmtSpeed(v?: number | null): string {
  if (v === undefined || v === null || v <= 0) return '-'
  return `${v.toFixed(2)} MB/s`
}

export function fmtPercent(v?: number | null): string {
  if (v === undefined || v === null) return '-'
  return `${(v * 100).toFixed(1)}%`
}

export async function copyText(text: string, tip?: string) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
    } else {
      // 非 HTTPS 环境下的回退
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    toast.success(tip ?? t('common.copied'))
  } catch {
    toast.error(t('format.copyFailed'))
  }
}

export type Tone = 'primary' | 'success' | 'warning' | 'danger' | 'info' | 'neutral'

// label 用 getter 按当前语言取文案：模板/computed 中读取时随语言切换而更新，调用方仍按对象下标访问
const labeled = (key: string, type: Tone) => ({
  get label() {
    return t(key)
  },
  type,
})

export const runStatusMeta: Record<RunStatus, { readonly label: string; type: Tone }> = {
  queued: labeled('format.status.queued', 'neutral'),
  running: labeled('format.status.running', 'primary'),
  success: labeled('format.status.success', 'success'),
  partial: labeled('format.status.partial', 'warning'),
  failed: labeled('format.status.failed', 'danger'),
  canceled: labeled('format.status.canceled', 'neutral'),
}

export const runTriggerLabel: Record<RunTrigger, string> = {
  get manual() {
    return t('format.trigger.manual')
  },
  get cron() {
    return t('format.trigger.cron')
  },
  hook: 'Webhook',
}

export const changeActionMeta: Record<DNSChangeAction, { readonly label: string; type: Tone }> = {
  create: labeled('format.action.create', 'success'),
  update: labeled('format.action.update', 'info'),
  delete: labeled('format.action.delete', 'warning'),
  skip: labeled('format.action.skip', 'neutral'),
  error: labeled('format.action.error', 'danger'),
}

export const ipTypeLabel: Record<IPType, string> = {
  v4: 'IPv4',
  v6: 'IPv6',
  both: 'IPv4 + IPv6',
}

export function isRunActive(s?: RunStatus | null) {
  return s === 'queued' || s === 'running'
}

/** 触发浏览器下载 */
export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

// ---------- 执行 / DNS 变更说明 ----------
// 后端在 messageKey 中给出固定说明的代码；早期记录只有中文原文，按已知文案反查以便同样随语言显示；
// 上游服务商报错等没有固定文案的内容原样显示。
const LEGACY_RUN_MESSAGES: Record<string, string> = {
  已取消: 'canceled',
  '试运行完成，未修改 DNS': 'dryRunDone',
  'DNS 记录已更新': 'dnsUpdated',
  'IP 未变化': 'ipUnchanged',
  '服务重启，执行被中断': 'interrupted',
  任务未配置目标记录: 'noTargets',
  '没有获得任何可用 IP，DNS 记录未修改': 'noIP',
  '全部 DNS 记录更新失败': 'allFailed',
}

interface Described {
  message?: string
  messageKey?: string
  messageArgs?: Record<string, unknown>
}

export function runMessage(r: Described): string {
  if (r.messageKey) return t(`messages.run.${r.messageKey}`, r.messageArgs)
  const m = r.message || ''
  const key = LEGACY_RUN_MESSAGES[m]
  if (key) return t(`messages.run.${key}`)
  const partial = /^(\d+)\/(\d+) 条记录更新失败$/.exec(m)
  if (partial) return t('messages.run.partialFailed', { failed: Number(partial[1]), total: Number(partial[2]) })
  return m
}

export function changeMessage(c: Described): string {
  if (c.messageKey) return t(`messages.change.${c.messageKey}`, c.messageArgs)
  const m = c.message || ''
  if (m === '无可用 IP，保留原记录') return t('messages.change.keptNoIP')
  const acc = /^DNS 账号不可用: (.*)$/s.exec(m)
  if (acc) return t('messages.change.accountUnavailable', { error: acc[1] })
  return m
}
