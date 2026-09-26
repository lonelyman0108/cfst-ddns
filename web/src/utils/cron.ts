import { t } from '@/i18n'

export interface CronPreset {
  readonly label: string
  value: string
}

// label 用 getter 按当前语言取文案，切换语言后模板中立即更新
const preset = (key: string, value: string): CronPreset => ({
  get label() {
    return t(`cron.presets.${key}`)
  },
  value,
})

/** 执行周期预设（5 段 cron；空字符串表示仅手动） */
export const CRON_PRESETS: CronPreset[] = [
  preset('every30m', '*/30 * * * *'),
  preset('hourly', '0 * * * *'),
  preset('every6h', '0 */6 * * *'),
  preset('every12h', '0 */12 * * *'),
  preset('daily3', '0 3 * * *'),
  preset('manual', ''),
]

const pad = (n: number) => String(n).padStart(2, '0')
const isNum = (s: string) => /^\d+$/.test(s)

/** 把常见 cron 表达式翻译为当前语言的描述，无法识别时返回原文 */
export function describeCron(expr: string): string {
  const e = (expr || '').trim()
  if (!e) return t('cron.presets.manual')
  const hit = CRON_PRESETS.find((p) => p.value === e)
  if (hit) return hit.label
  if (e.startsWith('@')) {
    const map: Record<string, string> = {
      '@hourly': 'cron.presets.hourly',
      '@daily': 'cron.at.daily0',
      '@midnight': 'cron.at.daily0',
      '@weekly': 'cron.at.weekly',
      '@monthly': 'cron.at.monthly',
      '@yearly': 'cron.at.yearly',
      '@annually': 'cron.at.yearly',
    }
    if (map[e]) return t(map[e])
    const every = /^@every\s+(.+)$/.exec(e)
    if (every) return t('cron.everyDuration', { d: every[1] })
    return e
  }
  const parts = e.split(/\s+/)
  if (parts.length !== 5) return e
  const [min, hour, dom, mon, dow] = parts
  const anyDate = dom === '*' && mon === '*'

  // */N * * * *
  let m = /^\*\/(\d+)$/.exec(min)
  if (m && hour === '*' && anyDate && dow === '*') return t('cron.everyNMin', { n: m[1] })
  if (min === '*' && hour === '*' && anyDate && dow === '*') return t('cron.everyMinute')
  // M */N * * *
  m = /^\*\/(\d+)$/.exec(hour)
  if (isNum(min) && m && anyDate && dow === '*') {
    return Number(min) === 0 ? t('cron.everyNHour', { n: m[1] }) : t('cron.everyNHourAt', { n: m[1], m: min })
  }
  // M * * * *
  if (isNum(min) && hour === '*' && anyDate && dow === '*') return t('cron.hourlyAt', { m: min })
  // M H * * *
  if (isNum(min) && isNum(hour) && anyDate && dow === '*') return t('cron.dailyAt', { time: `${pad(+hour)}:${pad(+min)}` })
  // M H1,H2 * * *
  if (isNum(min) && /^\d+(,\d+)+$/.test(hour) && anyDate && dow === '*') {
    const times = hour
      .split(',')
      .map((h) => `${pad(+h)}:${pad(+min)}`)
      .join(t('cron.sep'))
    return t('cron.dailyAt', { time: times })
  }
  // M H * * D
  if (isNum(min) && isNum(hour) && anyDate && isNum(dow)) {
    const day = +dow <= 7 ? t(`cron.weekday.${+dow % 7}`) : dow
    return t('cron.weeklyAt', { day, time: `${pad(+hour)}:${pad(+min)}` })
  }
  // M H D * *
  if (isNum(min) && isNum(hour) && isNum(dom) && mon === '*' && dow === '*') {
    return t('cron.monthlyAt', { d: dom, time: `${pad(+hour)}:${pad(+min)}` })
  }
  return e
}
