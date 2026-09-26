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
