import type { Component } from 'vue'
import { Bell, CircleHelp, CloudDownload, History, LayoutDashboard, ListChecks, ScrollText, Settings, UserRoundKey } from '@lucide/vue'

export interface NavItem {
  path: string
  title: string
  icon: Component
  group: 'main' | 'config' | 'system'
}

export const NAV: NavItem[] = [
  { path: '/', title: '仪表盘', icon: LayoutDashboard, group: 'main' },
  { path: '/tasks', title: '任务', icon: ListChecks, group: 'main' },
  { path: '/runs', title: '执行历史', icon: History, group: 'main' },
  { path: '/accounts', title: 'DNS 账号', icon: UserRoundKey, group: 'config' },
  { path: '/notifiers', title: '通知渠道', icon: Bell, group: 'config' },
  { path: '/cfst', title: 'cfst 管理', icon: CloudDownload, group: 'config' },
  { path: '/settings', title: '系统设置', icon: Settings, group: 'system' },
  { path: '/logs', title: '系统日志', icon: ScrollText, group: 'system' },
  { path: '/help', title: '帮助', icon: CircleHelp, group: 'system' },
]

export const NAV_GROUPS: { key: NavItem['group']; label: string }[] = [
  { key: 'main', label: '概览' },
  { key: 'config', label: '配置' },
  { key: 'system', label: '系统' },
]
