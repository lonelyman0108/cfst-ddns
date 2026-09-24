const KEY = 'cfst-ddns-token'
const USER_KEY = 'cfst-ddns-user'

export function getToken(): string {
  try {
    return localStorage.getItem(KEY) || ''
  } catch {
    return ''
  }
}

export function setToken(token: string, username: string) {
  try {
    localStorage.setItem(KEY, token)
    localStorage.setItem(USER_KEY, username)
  } catch {
    /* 忽略 */
  }
}

export function getStoredUsername(): string {
  try {
    return localStorage.getItem(USER_KEY) || ''
  } catch {
    return ''
  }
}

export function clearToken() {
  try {
    localStorage.removeItem(KEY)
    localStorage.removeItem(USER_KEY)
  } catch {
    /* 忽略 */
  }
}
