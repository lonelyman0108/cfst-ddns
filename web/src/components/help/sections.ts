import type { Component } from 'vue'
import { BookOpen, CircleHelp, DatabaseBackup, Rocket, SlidersHorizontal, Webhook } from '@lucide/vue'

export interface HelpSection {
  id: string
  title: string
  icon: Component
  /** 命令面板检索用的附加关键词 */
  keywords: string
}

export const HELP_SECTIONS: HelpSection[] = [
  { id: 'quick-start', title: '快速上手', icon: Rocket, keywords: '入门 开始 引导 向导 教程' },
  { id: 'cfst-params', title: 'cfst 参数说明', icon: SlidersHorizontal, keywords: '测速 参数 线程 延迟 下载 -n -t -tl -sl -dd httping' },
  { id: 'faq', title: '常见问题', icon: CircleHelp, keywords: 'faq 代理 host 网络 下载速度 0 忘记密码 docker' },
  { id: 'webhook', title: 'Webhook 用法', icon: Webhook, keywords: 'hook 触发 curl 外部 api' },
  { id: 'backup', title: '备份与迁移', icon: DatabaseBackup, keywords: '备份 恢复 迁移 v1 导入 secret.key' },
]

export const HELP_ICON = BookOpen
