import type { Component } from 'vue'
import { Bell, CloudDownload, ListChecks, Play, UserRoundKey } from '@lucide/vue'
import type { OnboardingKey, OnboardingState } from '@/api/types-p9'

export interface OnboardingStep {
  key: OnboardingKey
  /** 文案 key，使用处 t() */
  title: string
  description: string
  icon: Component
  optional?: boolean
}

export const ONBOARDING_STEPS: OnboardingStep[] = [
  { key: 'cfst', title: 'onboarding.steps.cfst.title', description: 'onboarding.steps.cfst.description', icon: CloudDownload },
  { key: 'account', title: 'onboarding.steps.account.title', description: 'onboarding.steps.account.description', icon: UserRoundKey },
  { key: 'notifier', title: 'onboarding.steps.notifier.title', description: 'onboarding.steps.notifier.description', icon: Bell, optional: true },
  { key: 'task', title: 'onboarding.steps.task.title', description: 'onboarding.steps.task.description', icon: ListChecks },
  { key: 'run', title: 'onboarding.steps.run.title', description: 'onboarding.steps.run.description', icon: Play },
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
