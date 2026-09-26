import { ref } from 'vue'
import { createI18n, type Composer } from 'vue-i18n'
// 直接引用 dayjs 而非 @/utils/format，避免与 format.ts（依赖 t）形成循环导入
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import 'dayjs/locale/zh-tw'
import 'dayjs/locale/ja'
import zhCN from '@/locales/zh-CN'
import zhTW from '@/locales/zh-TW'
import en from '@/locales/en'
import ja from '@/locales/ja'

export const LOCALES = [
  { value: 'zh-CN', label: '简体中文', dayjs: 'zh-cn' },
  { value: 'zh-TW', label: '繁體中文', dayjs: 'zh-tw' },
  { value: 'en', label: 'English', dayjs: 'en' },
  { value: 'ja', label: '日本語', dayjs: 'ja' },
] as const

export type Locale = (typeof LOCALES)[number]['value']

const STORAGE_KEY = 'cfst-ddns-locale'

/** 浏览器语言 → 支持的语言；繁体（台湾/香港/澳门/Hant）归为 zh-TW，其余中文归为 zh-CN */
function fromNavigator(): Locale {
  for (const raw of navigator.languages?.length ? navigator.languages : [navigator.language]) {
    const l = (raw || '').toLowerCase()
    if (/^zh-(tw|hk|mo)|hant/.test(l)) return 'zh-TW'
    if (l.startsWith('zh')) return 'zh-CN'
    if (l.startsWith('ja')) return 'ja'
    if (l.startsWith('en')) return 'en'
  }
  return 'en'
}

/** 用户选择：具体语言，或 auto（跟随浏览器，默认） */
export type LocalePreference = Locale | 'auto'

function loadPreference(): LocalePreference {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (LOCALES.some((l) => l.value === saved)) return saved as Locale
  } catch {
    /* 隐私模式等场景读取失败时跟随浏览器 */
  }
  return 'auto'
}

export const localePreference = ref<LocalePreference>(loadPreference())

/** 浏览器语言对应的界面语言（「跟随浏览器」时使用） */
export const browserLocale = ref<Locale>(fromNavigator())

const resolve = (p: LocalePreference): Locale => (p === 'auto' ? browserLocale.value : p)

// 文案按模块动态合并，无法静态推导结构，这里放宽为任意消息对象
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Messages = Record<string, any>

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: resolve(localePreference.value),
  fallbackLocale: 'zh-CN',
  messages: { 'zh-CN': zhCN, 'zh-TW': zhTW, en, ja } as Record<Locale, Messages>,
  missingWarn: false,
  fallbackWarn: false,
})

// 全局 Composer 的完整泛型会让类型检查展开过深，组件外统一经由这个简化类型访问
const g = i18n.global as unknown as Composer

/** 在组件外使用（api、store、工具函数）；组件内用 useI18n() */
export function t(key: string, named?: Record<string, unknown>): string {
  return named ? g.t(key, named) : g.t(key)
}

export function currentLocale(): Locale {
  return g.locale.value as Locale
}

function apply(l: Locale) {
  document.documentElement.lang = l
  dayjs.locale(LOCALES.find((x) => x.value === l)?.dayjs ?? 'en')
}

export function setLocalePreference(p: LocalePreference) {
  localePreference.value = p
  try {
    if (p === 'auto') localStorage.removeItem(STORAGE_KEY)
    else localStorage.setItem(STORAGE_KEY, p)
  } catch {
    /* 忽略 */
  }
  const l = resolve(p)
  g.locale.value = l
  apply(l)
}

// 跟随浏览器时，浏览器语言设置变化后同步切换
window.addEventListener('languagechange', () => {
  browserLocale.value = fromNavigator()
  if (localePreference.value === 'auto') setLocalePreference('auto')
})

apply(currentLocale())
