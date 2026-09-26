<script lang="ts">
import type { Component } from 'vue'

export interface NavSection {
  id: string
  title: string
  icon?: Component
}
</script>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

/** 分节页面布局：桌面端左侧吸顶目录（随滚动高亮），移动端顶部横向按钮；锚点可深链 */
const props = defineProps<{ sections: NavSection[] }>()

const route = useRoute()
const router = useRouter()
const active = ref(props.sections[0]?.id ?? '')

/** 与 router scrollBehavior 的 top 偏移一致（吸顶页头 56px + 间距） */
const OFFSET = 72
let raf = 0
/** 点击目录或锚点进入时的目标节；页面到底后末几节无法滚到顶部，以它为准，用户手动滚动后清除 */
let pinned = ''

function spy() {
  raf = 0
  let cur = props.sections[0]?.id ?? ''
  for (const s of props.sections) {
    const el = document.getElementById(s.id)
    if (el && el.getBoundingClientRect().top <= OFFSET + 24) cur = s.id
  }
  // 滚到底部时末几节无法到达顶部：优先保持跳转目标，否则高亮最后一节
  if (window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 2) {
    const top = pinned ? document.getElementById(pinned)?.getBoundingClientRect().top : undefined
    cur = top !== undefined && top < window.innerHeight ? pinned : (props.sections[props.sections.length - 1]?.id ?? cur)
  }
  active.value = cur
}

function onScroll() {
  if (!raf) raf = requestAnimationFrame(spy)
}

function scrollToId(id: string, smooth: boolean) {
  const el = document.getElementById(id)
  if (!el) return
  window.scrollTo({ top: el.getBoundingClientRect().top + window.scrollY - OFFSET, behavior: smooth ? 'smooth' : 'auto' })
}

function unpin() {
  pinned = ''
}

function jump(id: string) {
  pinned = id
  active.value = id
  // 地址栏同步锚点，但滚动自己处理（重复点击同一锚点时路由不会触发滚动）
  if (route.hash !== `#${id}`) router.replace({ hash: `#${id}` })
  scrollToId(id, true)
}

onMounted(() => {
  window.addEventListener('scroll', onScroll, { passive: true })
  for (const ev of ['wheel', 'touchstart', 'keydown'] as const) window.addEventListener(ev, unpin, { passive: true })
  // 懒加载页面首次渲染时路由可能找不到锚点，这里补一次
  if (route.hash) {
    pinned = route.hash.slice(1)
    nextTick(() => scrollToId(pinned, false))
  }
  nextTick(spy)
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScroll)
  for (const ev of ['wheel', 'touchstart', 'keydown'] as const) window.removeEventListener(ev, unpin)
  if (raf) cancelAnimationFrame(raf)
})
</script>

<template>
  <div class="grid gap-6 lg:grid-cols-[180px_1fr]">
    <nav class="hidden lg:block" aria-label="页面目录">
      <ul class="sticky top-20 grid gap-0.5 text-sm">
        <li v-for="s in sections" :key="s.id">
          <button
            type="button"
            :aria-current="active === s.id ? 'location' : undefined"
            :class="
              cn(
                'hover:bg-accent/60 flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors',
                active === s.id ? 'text-foreground bg-accent/60 font-medium' : 'text-muted-foreground',
              )
            "
            @click="jump(s.id)"
          >
            <component :is="s.icon" v-if="s.icon" class="size-4" />{{ s.title }}
          </button>
        </li>
      </ul>
    </nav>

    <div class="flex min-w-0 flex-col gap-4">
      <div class="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 lg:hidden">
        <Button
          v-for="s in sections"
          :key="s.id"
          :variant="active === s.id ? 'secondary' : 'outline'"
          size="sm"
          class="shrink-0"
          @click="jump(s.id)"
        >
          {{ s.title }}
        </Button>
      </div>
      <slot />
    </div>
  </div>
</template>
