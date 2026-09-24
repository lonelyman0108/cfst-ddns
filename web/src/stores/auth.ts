import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api'
import type { ChangePasswordRequest, Credentials, LoginResult } from '@/api/types'
import { clearToken, getStoredUsername, getToken, setToken } from '@/utils/token'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(getToken())
  const username = ref(getStoredUsername())
  /** null 表示尚未查询 */
  const initialized = ref<boolean | null>(null)

  const loggedIn = computed(() => !!token.value)

  function apply(res: LoginResult) {
    token.value = res.token
    username.value = res.username
    initialized.value = true
    setToken(res.token, res.username)
  }

  async function fetchStatus() {
    const s = await authApi.status()
    initialized.value = s.initialized
    return s.initialized
  }

  async function login(c: Credentials) {
    apply(await authApi.login(c))
  }

  async function setup(c: Credentials) {
    apply(await authApi.setup(c))
  }

  async function fetchMe() {
    const me = await authApi.me()
    username.value = me.username
    setToken(token.value, me.username)
  }

  /** 修改密码：旧令牌失效，改用响应中的新令牌 */
  async function changePassword(body: ChangePasswordRequest) {
    const res = await authApi.changePassword(body)
    if (res.token) apply(res)
  }

  /** 仅清除本地状态（401 或主动退出） */
  function reset() {
    token.value = ''
    username.value = ''
    clearToken()
  }

  return { token, username, initialized, loggedIn, fetchStatus, login, setup, fetchMe, changePassword, reset }
})
