import { get, post, put } from './http'
import type { AxiosProgressEvent } from 'axios'
import type { CfstRelease, CfstStatus, IPFile, IPFileKind, OkResponse } from './types'
import type { CfstCandidate, CfstImportResult, MirrorPreset, MirrorProbe } from './types-p9'

export const cfstApi = {
  status: () => get<CfstStatus>('/api/cfst'),
  releases: () => get<CfstRelease[]>('/api/cfst/releases', { timeout: 60000 }),
  /** 同步长请求（后端最长约 3 分钟） */
  install: (version: string) => post<{ version: string }>('/api/cfst/install', { version }, { timeout: 300000 }),
  getIPFile: (kind: IPFileKind) => get<IPFile>(`/api/cfst/ipfile/${kind}`),
  saveIPFile: (kind: IPFileKind, content: string) => put<OkResponse>(`/api/cfst/ipfile/${kind}`, { content }),
  resetIPFile: (kind: IPFileKind) => post<IPFile>(`/api/cfst/ipfile/${kind}/reset`),
  /** 上传压缩包或裸可执行文件导入；onProgress 回调 0–100 */
  upload: (file: File, version?: string, onProgress?: (percent: number) => void) => {
    const fd = new FormData()
    fd.append('file', file)
    if (version) fd.append('version', version)
    return post<CfstImportResult>('/api/cfst/upload', fd, {
      timeout: 600000,
      onUploadProgress: (e: AxiosProgressEvent) => {
        if (onProgress && e.total) onProgress(Math.round((e.loaded / e.total) * 100))
      },
    })
  },
  /** 查找本机已有的 cfst（数据目录、PATH、镜像内置目录） */
  scan: () => get<CfstCandidate[]>('/api/cfst/scan', { timeout: 60000 }),
  adopt: (path: string, version?: string) => post<CfstImportResult>('/api/cfst/adopt', { path, version: version || undefined }, { timeout: 60000 }),
  mirrors: () => get<MirrorPreset[]>('/api/cfst/mirrors'),
  /** 省略 mirrors 时测全部预设 + 当前设置值 */
  testMirrors: (mirrors?: string[]) => post<MirrorProbe[]>('/api/cfst/mirrors/test', mirrors ? { mirrors } : {}, { timeout: 60000 }),
}
