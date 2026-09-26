import { currentLocale } from '@/i18n'
import { del, get, post } from './http'
import type { DeletedCount, OkResponse, RunDetail, RunList, RunListQuery, RunSummary } from './types'
import { getToken } from '@/utils/token'

export const runsApi = {
  list: (q: RunListQuery) => get<RunList>('/api/runs', { params: q }),
  active: () => get<RunSummary[]>('/api/runs/active'),
  get: (id: number) => get<RunDetail>(`/api/runs/${id}`),
  cancel: (id: number) => post<OkResponse>(`/api/runs/${id}/cancel`),
  remove: (id: number) => del<OkResponse>(`/api/runs/${id}`),
  purge: (beforeDays: number) => del<DeletedCount>('/api/runs', { params: { beforeDays } }),
  /** EventSource 无法设置请求头，令牌通过查询参数传递 */
  streamUrl: (id: number) => `/api/runs/${id}/stream?token=${encodeURIComponent(getToken())}&lang=${currentLocale()}`,
}
