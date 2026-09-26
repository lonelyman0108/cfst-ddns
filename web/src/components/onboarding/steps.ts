import type { Component } from 'vue'
import { Bell, CloudDownload, ListChecks, Play, UserRoundKey } from '@lucide/vue'
import type { OnboardingKey, OnboardingState } from '@/api/types-p9'

export interface OnboardingStep {
  key: OnboardingKey
  title: string
  description: string
  icon: Component
  optional?: boolean
}

export const ONBOARDING_STEPS: OnboardingStep[] = [
  { key: 'cfst', title: '准备 cfst', description: '安装测速程序 CloudflareSpeedTest', icon: CloudDownload },
  { key: 'account', title: '添加 DNS 账号', description: '填写服务商凭据并测试连接', icon: UserRoundKey },
  { key: 'notifier', title: '通知渠道', description: '可选：测速结果推送到手机', icon: Bell, optional: true },
  { key: 'task', title: '创建任务', description: '从模板创建第一个测速任务', icon: ListChecks },
  { key: 'run', title: '首次执行', description: '试运行或正式执行并查看日志', icon: Play },
]

/** 第一个未完成的步骤（通知为可选，任务已建时视为跳过） */
export function firstPendingStep(s: OnboardingState): number {
  if (!s.cfst) return 0
  if (!s.account) return 1
  if (!s.task) return s.notifier ? 3 : 2
  if (!s.run) return 4
  return 4
}

export function stepIndex(key: unknown): number {
  return ONBOARDING_STEPS.findIndex((s) => s.key === key)
}
