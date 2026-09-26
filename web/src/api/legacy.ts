import { post } from './http'
import type { LegacyApplyResult, LegacyPreview } from './types-p9'

export const legacyApi = {
  /** 只解析、不写入 */
  preview: (content: string) => post<LegacyPreview>('/api/import/legacy', { content, apply: false }, { timeout: 60000 }),
  apply: (content: string) => post<LegacyApplyResult>('/api/import/legacy', { content, apply: true }, { timeout: 120000 }),
}
