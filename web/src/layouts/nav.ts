import type { Component } from 'vue'
import { Bell, CircleHelp, CloudDownload, History, LayoutDashboard, ListChecks, ScrollText, Settings, UserRoundKey } from '@lucide/vue'

export interface NavItem {
  path: string
  /** 文案 key，使用处 t() */
  title: string
  icon: Component
  group: 'main' | 'config' | 'system'
}

export const NAV: NavItem[] = [
  { path: '/', title: 'nav.dashboard', icon: LayoutDashboard, group: 'main' },
  { path: '/tasks', title: 'nav.tasks', icon: ListChecks, group: 'main' },
  { path: '/runs', title: 'nav.runs', icon: History, group: 'main' },
  { path: '/accounts', title: 'nav.accounts', icon: UserRoundKey, group: 'config' },
  { path: '/notifiers', title: 'nav.notifiers', icon: Bell, group: 'config' },
  { path: '/cfst', title: 'nav.cfst', icon: CloudDownload, group: 'config' },
  { path: '/settings', title: 'nav.settings', icon: Settings, group: 'system' },
  { path: '/logs', title: 'nav.logs', icon: ScrollText, group: 'system' },
  { path: '/help', title: 'nav.help', icon: CircleHelp, group: 'system' },
]

// label 为文案 key
export const NAV_GROUPS: { key: NavItem['group']; label: string }[] = [
  { key: 'main', label: 'nav.groups.main' },
  { key: 'config', label: 'nav.groups.config' },
  { key: 'system', label: 'nav.groups.system' },
]
