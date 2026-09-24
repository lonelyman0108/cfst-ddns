import { get } from './http'
import type { Dashboard } from './types'

export const dashboardApi = {
  get: (silent = false) => get<Dashboard>('/api/dashboard', { silent }),
}
