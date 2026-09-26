import type { Component } from 'vue'
import { BookOpen, CircleHelp, DatabaseBackup, Rocket, SlidersHorizontal, Webhook } from '@lucide/vue'

export interface HelpSection {
  id: string
  /** 文案 key，使用处 t() */
  title: string
  icon: Component
  /** 命令面板检索用的附加关键词（中英文并列，任何语言下都能搜到） */
  keywords: string
}

export const HELP_SECTIONS: HelpSection[] = [
  { id: 'quick-start', title: 'help.sections.quickStart', icon: Rocket, keywords: '入门 开始 引导 向导 教程 getting started guide wizard tutorial' },
  { id: 'cfst-params', title: 'help.sections.cfstParams', icon: SlidersHorizontal, keywords: '测速 参数 线程 延迟 下载 speed test parameters threads latency download -n -t -tl -sl -dd httping' },
  { id: 'faq', title: 'help.sections.faq', icon: CircleHelp, keywords: 'faq 代理 host 网络 下载速度 0 忘记密码 docker proxy network download speed forgot password reset' },
  { id: 'webhook', title: 'help.sections.webhook', icon: Webhook, keywords: 'hook 触发 curl 外部 api trigger external' },
  { id: 'backup', title: 'help.sections.backup', icon: DatabaseBackup, keywords: '备份 恢复 迁移 v1 导入 secret.key backup restore migrate import' },
]

export const HELP_ICON = BookOpen
