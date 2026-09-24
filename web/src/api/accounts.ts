import { del, get, post, put } from './http'
import type { Account, AccountCreate, AccountUpdate, Config, DNSRecord, OkResponse, TestResult } from './types'

export const accountsApi = {
  list: () => get<Account[]>('/api/accounts'),
  create: (body: AccountCreate) => post<Account>('/api/accounts', body),
  update: (id: number, body: AccountUpdate) => put<Account>(`/api/accounts/${id}`, body),
  /** 被任务引用时返回 409 */
  remove: (id: number) => del<OkResponse>(`/api/accounts/${id}`),
  /** 编辑已有账号时传 id，值为 ****** 的密钥由后端用已保存值补齐 */
  test: (body: { provider: string; config: Config; id?: number }) =>
    post<TestResult>('/api/accounts/test', body, { timeout: 60000 }),
  testSaved: (id: number) => post<TestResult>(`/api/accounts/${id}/test`, undefined, { timeout: 60000 }),
  domains: (id: number) => get<string[]>(`/api/accounts/${id}/domains`, { silent: true, timeout: 60000 }),
  records: (id: number, domain: string, rr?: string) =>
    get<DNSRecord[]>(`/api/accounts/${id}/records`, {
      params: rr ? { domain, rr } : { domain },
      timeout: 60000,
    }),
}
