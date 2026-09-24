import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'
import { toast } from 'vue-sonner'
import type { DNSChangeAction, IPType, RunStatus, RunTrigger } from '@/api/types'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

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
  if (s < 60) return `${s} 秒`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m} 分 ${s % 60} 秒`
  return `${Math.floor(m / 60)} 时 ${m % 60} 分`
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

export async function copyText(text: string, tip = '已复制') {
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
    toast.success(tip)
  } catch {
    toast.error('复制失败，请手动复制')
  }
}

export type Tone = 'primary' | 'success' | 'warning' | 'danger' | 'info' | 'neutral'

export const runStatusMeta: Record<RunStatus, { label: string; type: Tone }> = {
  queued: { label: '排队中', type: 'neutral' },
  running: { label: '运行中', type: 'primary' },
  success: { label: '成功', type: 'success' },
  partial: { label: '部分成功', type: 'warning' },
  failed: { label: '失败', type: 'danger' },
  canceled: { label: '已取消', type: 'neutral' },
}

export const runTriggerLabel: Record<RunTrigger, string> = {
  manual: '手动',
  cron: '定时',
  hook: 'Webhook',
}

export const changeActionMeta: Record<DNSChangeAction, { label: string; type: Tone }> = {
  create: { label: '新建', type: 'success' },
  update: { label: '更新', type: 'info' },
  delete: { label: '删除', type: 'warning' },
  skip: { label: '跳过', type: 'neutral' },
  error: { label: '错误', type: 'danger' },
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
