export interface CronPreset {
  label: string
  value: string
}

/** 执行周期预设（5 段 cron；空字符串表示仅手动） */
export const CRON_PRESETS: CronPreset[] = [
  { label: '每 30 分钟', value: '*/30 * * * *' },
  { label: '每小时', value: '0 * * * *' },
  { label: '每 6 小时', value: '0 */6 * * *' },
  { label: '每 12 小时', value: '0 */12 * * *' },
  { label: '每天 3 点', value: '0 3 * * *' },
  { label: '仅手动', value: '' },
]

const pad = (n: number) => String(n).padStart(2, '0')
const isNum = (s: string) => /^\d+$/.test(s)
const WEEK = ['日', '一', '二', '三', '四', '五', '六', '日']

/** 把常见 cron 表达式翻译为中文描述，无法识别时返回原文 */
export function describeCron(expr: string): string {
  const e = (expr || '').trim()
  if (!e) return '仅手动'
  const preset = CRON_PRESETS.find((p) => p.value === e)
  if (preset) return preset.label
  if (e.startsWith('@')) {
    const map: Record<string, string> = {
      '@hourly': '每小时',
      '@daily': '每天 0 点',
      '@midnight': '每天 0 点',
      '@weekly': '每周日 0 点',
      '@monthly': '每月 1 日 0 点',
      '@yearly': '每年 1 月 1 日 0 点',
      '@annually': '每年 1 月 1 日 0 点',
    }
    if (map[e]) return map[e]
    const every = /^@every\s+(.+)$/.exec(e)
    if (every) return `每隔 ${every[1]}`
    return e
  }
  const parts = e.split(/\s+/)
  if (parts.length !== 5) return e
  const [min, hour, dom, mon, dow] = parts
  const anyDate = dom === '*' && mon === '*'

  // */N * * * *
  let m = /^\*\/(\d+)$/.exec(min)
  if (m && hour === '*' && anyDate && dow === '*') return `每 ${m[1]} 分钟`
  if (min === '*' && hour === '*' && anyDate && dow === '*') return '每分钟'
  // M */N * * *
  m = /^\*\/(\d+)$/.exec(hour)
  if (isNum(min) && m && anyDate && dow === '*') {
    return Number(min) === 0 ? `每 ${m[1]} 小时` : `每 ${m[1]} 小时（第 ${min} 分）`
  }
  // M * * * *
  if (isNum(min) && hour === '*' && anyDate && dow === '*') return `每小时第 ${min} 分`
  // M H * * *
  if (isNum(min) && isNum(hour) && anyDate && dow === '*') return `每天 ${pad(+hour)}:${pad(+min)}`
  // M H1,H2 * * *
  if (isNum(min) && /^\d+(,\d+)+$/.test(hour) && anyDate && dow === '*') {
    return `每天 ${hour
      .split(',')
      .map((h) => `${pad(+h)}:${pad(+min)}`)
      .join('、')}`
  }
  // M H * * D
  if (isNum(min) && isNum(hour) && anyDate && isNum(dow)) {
    return `每周${WEEK[+dow] ?? dow} ${pad(+hour)}:${pad(+min)}`
  }
  // M H D * *
  if (isNum(min) && isNum(hour) && isNum(dom) && mon === '*' && dow === '*') {
    return `每月 ${dom} 日 ${pad(+hour)}:${pad(+min)}`
  }
  return e
}
