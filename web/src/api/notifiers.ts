import { del, get, post, put } from './http'
import type { Config, Notifier, NotifierInput, OkResponse, TestResult } from './types'

export const notifiersApi = {
  list: () => get<Notifier[]>('/api/notifiers'),
  create: (body: NotifierInput) => post<Notifier>('/api/notifiers', body),
  update: (id: number, body: NotifierInput) => put<Notifier>(`/api/notifiers/${id}`, body),
  remove: (id: number) => del<OkResponse>(`/api/notifiers/${id}`),
  /** 编辑已有渠道时传 id，值为 ****** 的密钥由后端用已保存值补齐 */
  test: (body: { type: string; config: Config; id?: number }) =>
    post<TestResult>('/api/notifiers/test', body, { timeout: 60000 }),
  testSaved: (id: number) => post<TestResult>(`/api/notifiers/${id}/test`, undefined, { timeout: 60000 }),
}
