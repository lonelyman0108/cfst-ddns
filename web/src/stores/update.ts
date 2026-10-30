import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { systemApi } from '@/api'
import type { UpdateInfo } from '@/api/types'

/** 新版本检查结果，侧栏与系统设置共用 */
export const useUpdateStore = defineStore('update', () => {
  const info = ref<UpdateInfo | null>(null)
  const checking = ref(false)
  const available = computed(() => !!info.value?.available)

  /** force 为 true 时立即检查；失败时保留上一次结果 */
  async function load(force = false) {
    checking.value = true
    try {
      info.value = await systemApi.update(force)
    } finally {
      checking.value = false
    }
  }

  return { info, checking, available, load }
})
