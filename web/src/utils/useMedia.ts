import { onBeforeUnmount, onMounted, ref } from 'vue'

/** 视口宽度是否小于给定值（响应式） */
export function useNarrow(max = 768) {
  const narrow = ref(typeof window !== 'undefined' ? window.innerWidth < max : false)
  const update = () => (narrow.value = window.innerWidth < max)
  onMounted(() => {
    update()
    window.addEventListener('resize', update)
  })
  onBeforeUnmount(() => window.removeEventListener('resize', update))
  return narrow
}

/** 页面可见时按间隔执行，不可见时暂停；恢复可见时立即执行一次 */
export function useVisibleInterval(fn: () => void, ms: number) {
  let timer: ReturnType<typeof setInterval> | undefined
  const start = () => {
    stop()
    timer = setInterval(() => {
      if (document.visibilityState === 'visible') fn()
    }, ms)
  }
  const stop = () => {
    if (timer) clearInterval(timer)
    timer = undefined
  }
  const onVis = () => {
    if (document.visibilityState === 'visible') fn()
  }
  onMounted(() => {
    start()
    document.addEventListener('visibilitychange', onVis)
  })
  onBeforeUnmount(() => {
    stop()
    document.removeEventListener('visibilitychange', onVis)
  })
  return { start, stop }
}
