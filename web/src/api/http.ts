import axios, { AxiosError, type AxiosRequestConfig } from 'axios'
import { toast } from 'vue-sonner'
import { clearToken, getToken } from '@/utils/token'
import { currentLocale, t } from '@/i18n'

declare module 'axios' {
  interface AxiosRequestConfig {
    /** 为 true 时出错不自动弹出提示，由调用方自行处理 */
    silent?: boolean
  }
}

const http = axios.create({
  baseURL: '/',
  timeout: 30000,
})

let unauthorizedHandler: (() => void) | null = null

/** 注册 401 处理（由路由初始化时设置，避免循环依赖） */
export function onUnauthorized(fn: () => void) {
  unauthorizedHandler = fn
}

http.interceptors.request.use((config) => {
  const token = getToken()
  if (token) config.headers.set('Authorization', `Bearer ${token}`)
  config.headers.set('Accept-Language', currentLocale())
  return config
})

async function extractMessage(err: AxiosError): Promise<string> {
  const data = err.response?.data as unknown
  if (data instanceof Blob) {
    try {
      const parsed = JSON.parse(await data.text()) as { error?: string }
      if (parsed?.error) return parsed.error
    } catch {
      /* 忽略 */
    }
  } else if (data && typeof data === 'object' && 'error' in data) {
    const msg = (data as { error?: unknown }).error
    if (typeof msg === 'string' && msg) return msg
  }
  if (err.code === 'ECONNABORTED') return t('format.http.timeout')
  if (!err.response) return t('format.http.network')
  return t('format.http.status', { status: err.response.status })
}

/** 从任意错误中取出可读信息（优先使用后端返回的 error 字段） */
export function errorMessage(e: unknown): string {
  if (e && typeof e === 'object' && '__message' in e) return String((e as { __message: string }).__message)
  if (e instanceof Error) return e.message
  return String(e)
}

/** 取 HTTP 状态码 */
export function errorStatus(e: unknown): number | undefined {
  return axios.isAxiosError(e) ? e.response?.status : undefined
}

const AUTH_PUBLIC = ['/api/auth/login', '/api/auth/setup', '/api/auth/status']

http.interceptors.response.use(
  (res) => res,
  async (err: AxiosError) => {
    if (axios.isCancel(err)) return Promise.reject(err)
    const cfg = (err.config || {}) as AxiosRequestConfig
    const msg = await extractMessage(err)
    Object.assign(err, { __message: msg })
    const url = cfg.url || ''
    if (err.response?.status === 401 && !AUTH_PUBLIC.some((p) => url.startsWith(p))) {
      clearToken()
      unauthorizedHandler?.()
      return Promise.reject(err)
    }
    if (!cfg.silent) toast.error(msg)
    return Promise.reject(err)
  },
)

export async function get<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  return (await http.get<T>(url, config)).data
}
export async function post<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
  return (await http.post<T>(url, data, config)).data
}
export async function put<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
  return (await http.put<T>(url, data, config)).data
}
export async function patch<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
  return (await http.patch<T>(url, data, config)).data
}
export async function del<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  return (await http.delete<T>(url, config)).data
}

export default http
