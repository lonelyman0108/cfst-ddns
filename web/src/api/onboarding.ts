import { accountsApi } from './accounts'
import { cfstApi } from './cfst'
import { notifiersApi } from './notifiers'
import { runsApi } from './runs'
import { tasksApi } from './tasks'
import type { OnboardingState } from './types-p9'

export const onboardingApi = {
  /** 由现有数据推导入门进度（无需后端新接口） */
  async state(): Promise<OnboardingState> {
    const [cfst, accounts, notifiers, tasks, runs] = await Promise.all([
      cfstApi.status().catch(() => null),
      accountsApi.list().catch(() => []),
      notifiersApi.list().catch(() => []),
      tasksApi.list().catch(() => []),
      runsApi.list({ status: 'success', page: 1, size: 1 }).catch(() => ({ items: [], total: 0 })),
    ])
    return {
      cfst: !!cfst?.installed,
      account: (accounts ?? []).length > 0,
      notifier: (notifiers ?? []).length > 0,
      task: (tasks ?? []).length > 0,
      run: (runs?.total ?? 0) > 0 || (runs?.items ?? []).length > 0,
    }
  },
}
