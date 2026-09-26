// 服务商与通知渠道的品牌标识。均为本地文件（内网/离线部署不能依赖 CDN），
// SVG 来源见各 .svg 文件首行注释；PNG 为官网/官方仓库图标缩放而来：
// bark ← github.com/Finb/Bark AppIcon，dnspod ← dnspod.cn favicon，pushplus ← pushplus.plus favicon，
// serverchan ← sct.ftqq.com favicon。

export interface Brand {
  /** 品牌主色，用于标识着色或首字母方块底色 */
  color: string
  /** 暗色模式下的替代色（主色在深色背景上对比度不足时） */
  darkColor?: string
  /** 本地 SVG 文件名（不含扩展名）；缺省时渲染首字母方块 */
  svg?: string
  /** 本地 PNG 文件名（不含扩展名）；用于只有位图官方图标的品牌 */
  img?: string
  /** 位图本身是带底色的应用图标，铺满方块显示 */
  fill?: boolean
  /** lucide 图标名；用于通用协议类渠道 */
  lucide?: 'webhook' | 'mail'
  /** 首字母方块的文字 */
  initial?: string
}

export const providerBrands: Record<string, Brand> = {
  cloudflare: { color: '#F38020', svg: 'cloudflare' },
  dnspod: { color: '#0073E6', img: 'dnspod' },
  tencentcloud: { color: '#006EFF', svg: 'tencentcloud' },
  alidns: { color: '#FF6A00', svg: 'alidns' },
  huaweicloud: { color: '#E60012', darkColor: '#FF4D5A', svg: 'huaweicloud' },
  godaddy: { color: '#111111', darkColor: '#1BDBDB', svg: 'godaddy' },
}

export const notifierBrands: Record<string, Brand> = {
  bark: { color: '#FF3B30', img: 'bark', fill: true },
  telegram: { color: '#26A5E4', svg: 'telegram' },
  wecom: { color: '#0082EF', darkColor: '#3DA5FF', svg: 'wecom' },
  dingtalk: { color: '#1677FF', darkColor: '#4096FF', svg: 'dingtalk' },
  feishu: { color: '#3370FF', darkColor: '#5B8CFF', svg: 'feishu' },
  serverchan: { color: '#0B5394', img: 'serverchan', fill: true },
  pushplus: { color: '#1E88E5', img: 'pushplus', fill: true },
  gotify: { color: '#3E9DC7', svg: 'gotify' },
  ntfy: { color: '#317F6F', darkColor: '#5CC2A9', svg: 'ntfy' },
  webhook: { color: '#71717A', darkColor: '#A1A1AA', lucide: 'webhook' },
  smtp: { color: '#71717A', darkColor: '#A1A1AA', lucide: 'mail' },
}

const images = import.meta.glob<string>('./*.png', { query: '?url', import: 'default', eager: true })

/** 按文件名取 PNG 地址（构建时打包进 dist） */
export function brandImg(name: string): string | undefined {
  return images[`./${name}.png`]
}

const raw = import.meta.glob<string>('./*.svg', { query: '?raw', import: 'default', eager: true })

/** 按文件名取 SVG 标记（去掉来源注释；显式加上尺寸与颜色类，避免被 SelectItem 等父级的 svg 选择器覆盖） */
export function brandSvg(name: string): string | undefined {
  return raw[`./${name}.svg`]
    ?.replace(/<!--[\s\S]*?-->/g, '')
    .replace('<svg ', '<svg class="size-[58%] text-current" ')
    .trim()
}
