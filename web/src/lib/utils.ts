import { type ClassValue, clsx } from 'clsx'
import { extendTailwindMerge } from 'tailwind-merge'

// 自定义字号 token（见 styles/main.css 的 @theme），需告知 tailwind-merge 它们属于字号而非颜色，
// 否则 text-label 与 text-muted-foreground 会被误判为冲突而丢掉颜色类
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      'font-size': [{ text: ['label', 'code', 'log', 'card-title', 'page-title', 'stat'] }],
    },
  },
})

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
