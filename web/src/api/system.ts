import { get } from './http'
import type { SystemInfo, SystemLogs, TypeMeta } from './types'

export const metaApi = {
  providers: () => get<TypeMeta[]>('/api/meta/providers'),
  notifiers: () => get<TypeMeta[]>('/api/meta/notifiers'),
}

export const systemApi = {
  info: () => get<SystemInfo>('/api/system/info'),
  logs: (limit = 500, silent = false) => get<SystemLogs>('/api/system/logs', { params: { limit }, silent }),
}
