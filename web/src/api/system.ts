import { get } from './http'
import type { SystemInfo, SystemLogs, TypeMeta, UpdateInfo } from './types'

export const metaApi = {
  providers: () => get<TypeMeta[]>('/api/meta/providers'),
  notifiers: () => get<TypeMeta[]>('/api/meta/notifiers'),
}

export const systemApi = {
  info: () => get<SystemInfo>('/api/system/info'),
  /** force 为 true 时立即向 GitHub 检查，否则可能返回缓存 */
  update: (force = false) => get<UpdateInfo>('/api/system/update', { params: force ? { force: 1 } : {}, silent: !force }),
  logs: (limit = 500, silent = false) => get<SystemLogs>('/api/system/logs', { params: { limit }, silent }),
}
