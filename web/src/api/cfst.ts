import { get, post, put } from './http'
import type { CfstRelease, CfstStatus, IPFile, IPFileKind, OkResponse } from './types'

export const cfstApi = {
  status: () => get<CfstStatus>('/api/cfst'),
  releases: () => get<CfstRelease[]>('/api/cfst/releases', { timeout: 60000 }),
  /** 同步长请求（后端最长约 3 分钟） */
  install: (version: string) => post<{ version: string }>('/api/cfst/install', { version }, { timeout: 300000 }),
  getIPFile: (kind: IPFileKind) => get<IPFile>(`/api/cfst/ipfile/${kind}`),
  saveIPFile: (kind: IPFileKind, content: string) => put<OkResponse>(`/api/cfst/ipfile/${kind}`, { content }),
  resetIPFile: (kind: IPFileKind) => post<IPFile>(`/api/cfst/ipfile/${kind}/reset`),
}
