import { defineStore } from 'pinia'
import { computed, ref, watchEffect } from 'vue'
import { usePreferredDark } from '@vueuse/core'

const KEY = 'cfst-ddns-theme'

export type ThemeMode = 'light' | 'dark' | 'system'

function initial(): ThemeMode {
  try {
    const t = localStorage.getItem(KEY)
    if (t === 'dark' || t === 'light' || t === 'system') return t
  } catch {
    /* 忽略 */
  }
  return 'system'
}

export const useThemeStore = defineStore('theme', () => {
  const mode = ref<ThemeMode>(initial())
  const systemDark = usePreferredDark()
  const dark = computed(() => (mode.value === 'system' ? systemDark.value : mode.value === 'dark'))

  function setMode(m: ThemeMode) {
    mode.value = m
    try {
      localStorage.setItem(KEY, m)
    } catch {
      /* 忽略 */
    }
  }

  function toggle() {
    setMode(dark.value ? 'light' : 'dark')
  }

  watchEffect(() => {
    document.documentElement.classList.toggle('dark', dark.value)
    document.documentElement.style.colorScheme = dark.value ? 'dark' : 'light'
  })

  return { mode, dark, setMode, toggle }
})
