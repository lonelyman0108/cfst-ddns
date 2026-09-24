import { get, post } from './http'
import type { AuthStatus, ChangePasswordRequest, ChangePasswordResult, Credentials, LoginResult, Me } from './types'

export const authApi = {
  status: () => get<AuthStatus>('/api/auth/status'),
  setup: (body: Credentials) => post<LoginResult>('/api/auth/setup', body),
  login: (body: Credentials) => post<LoginResult>('/api/auth/login', body),
  me: () => get<Me>('/api/auth/me'),
  changePassword: (body: ChangePasswordRequest) => post<ChangePasswordResult>('/api/auth/password', body),
}
