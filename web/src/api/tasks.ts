import { del, get, patch, post, put } from './http'
import type { CronPreview, OkResponse, RunStarted, Task, TaskInput } from './types'

export const tasksApi = {
  list: () => get<Task[]>('/api/tasks'),
  defaults: () => get<Task>('/api/tasks/defaults'),
  get: (id: number) => get<Task>(`/api/tasks/${id}`),
  create: (body: TaskInput) => post<Task>('/api/tasks', body),
  update: (id: number, body: TaskInput) => put<Task>(`/api/tasks/${id}`, body),
  setEnabled: (id: number, enabled: boolean) => patch<Task>(`/api/tasks/${id}/enabled`, { enabled }),
  remove: (id: number) => del<OkResponse>(`/api/tasks/${id}`),
  clone: (id: number) => post<Task>(`/api/tasks/${id}/clone`),
  /** 已在运行或排队时返回 409 */
  run: (id: number) => post<RunStarted>(`/api/tasks/${id}/run`),
}

export const cronApi = {
  preview: (expr: string) => get<CronPreview>('/api/cron/preview', { params: { expr }, silent: true }),
}
