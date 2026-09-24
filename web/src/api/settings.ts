import http, { get, post, put } from './http'
import type { OkResponse, Settings } from './types'

export const settingsApi = {
  get: () => get<Settings>('/api/settings'),
  /** hookToken 字段会被后端忽略 */
  update: (body: Partial<Settings>) => put<Settings>('/api/settings', body),
  regenerateHookToken: () => post<Settings>('/api/settings/hook-token'),
}

export const backupApi = {
  /** 带鉴权下载备份，返回 Blob 与服务端建议的文件名 */
  async download(): Promise<{ blob: Blob; filename: string }> {
    const res = await http.get<Blob>('/api/backup', { responseType: 'blob', timeout: 120000 })
    const cd = String(res.headers['content-disposition'] || '')
    const m = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(cd)
    return { blob: res.data, filename: m ? decodeURIComponent(m[1]) : '' }
  },
  restore: (data: unknown) => post<OkResponse>('/api/backup/restore', data, { timeout: 120000 }),
}
