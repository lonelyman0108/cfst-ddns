<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { ArrowDownToLine, Copy } from '@lucide/vue'
import { Switch } from '@/components/ui/switch'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'
import { copyText } from '@/utils/format'

const props = withDefaults(
  defineProps<{
    text: string
    progress?: string
    live?: boolean
    title?: string
    /** 日志区高度（CSS 值） */
    height?: string
  }>(),
  { progress: '', live: false, title: '', height: '420px' },
)

const box = ref<HTMLElement>()
const follow = ref(true)

interface Line {
  time: string
  body: string
  cls: string
}

// cfst 进度条形如「1101 / 5955 [----->____] 可用: 1100」：拆成计数、比例与尾部说明，
// 用自适应宽度的进度条代替字符画，窄屏也能完整显示；无法识别时按原文截断显示
const PROGRESS_RE = /^\s*(\d+)\s*\/\s*(\d+)\s*\[[^\]]*\]\s*(.*)$/
const bar = computed(() => {
  const m = PROGRESS_RE.exec(props.progress)
  if (!m) return null
  const done = Number(m[1])
  const total = Number(m[2])
  return { done, total, pct: total > 0 ? Math.min(100, (done / total) * 100) : 0, tail: m[3].trim() }
})

const TIME_RE = /^(\d{1,2}:\d{2}:\d{2}|\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2})\s(.*)$/

function classify(body: string): string {
  const b = body.trimStart()
  if (b.startsWith('✓') || b.startsWith('√')) return 'text-emerald-400'
  if (b.startsWith('✗') || b.startsWith('×') || /\bERROR\b|失败|错误/.test(b)) return 'text-red-400'
  if (b.startsWith('!') || b.startsWith('⚠') || /\bWARN(ING)?\b|警告/.test(b)) return 'text-amber-300'
  if (b.startsWith('$ ')) return 'text-sky-300/80'
  if (b.startsWith('━━')) return 'text-orange-300 font-semibold'
  if (b.startsWith('#')) return 'text-zinc-400'
  return ''
}

const lines = computed<Line[]>(() => {
  if (!props.text) return []
  return props.text.split('\n').map((raw) => {
    const m = TIME_RE.exec(raw)
    const time = m ? m[1] : ''
    const body = m ? m[2] : raw
    return { time, body, cls: classify(body) }
  })
})

function atBottom(el: HTMLElement) {
  return el.scrollTop + el.clientHeight >= el.scrollHeight - 24
}

function onScroll() {
  const el = box.value
  if (!el) return
  // 用户上滚时暂停自动滚动，回到底部后恢复
  follow.value = atBottom(el)
}

function toBottom() {
  const el = box.value
  if (el) el.scrollTop = el.scrollHeight
}

function setFollow(v: boolean) {
  follow.value = v
  if (v) nextTick(toBottom)
}

watch(
  () => [props.text, props.progress],
  async () => {
    if (!follow.value) return
    await nextTick()
    toBottom()
  },
  { immediate: true, flush: 'post' },
)
</script>

<template>
  <div class="bg-term text-term-foreground overflow-hidden rounded-lg border border-zinc-800 shadow-sm">
    <div class="flex items-center gap-2 border-b border-white/10 px-3 py-2 text-xs">
      <div class="flex gap-1.5">
        <span class="size-2.5 rounded-full bg-red-400/70" />
        <span class="size-2.5 rounded-full bg-amber-300/70" />
        <span class="size-2.5 rounded-full bg-emerald-400/70" />
      </div>
      <span class="ml-1 truncate text-zinc-400">{{ title }}</span>
      <span v-if="live" class="flex items-center gap-1.5 text-emerald-400">
        <span class="relative flex size-1.5">
          <span class="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400 opacity-75" />
          <span class="relative inline-flex size-1.5 rounded-full bg-emerald-400" />
        </span>
        实时
      </span>
      <div class="ml-auto flex items-center gap-3">
        <label class="flex cursor-pointer items-center gap-1.5 text-zinc-400 select-none">
          <Switch :model-value="follow" class="scale-90 data-[state=unchecked]:bg-zinc-700" @update:model-value="setFollow" />
          自动滚动
        </label>
        <Tooltip>
          <TooltipTrigger as-child>
            <button
              type="button"
              class="rounded p-1 text-zinc-400 transition-colors hover:bg-white/10 hover:text-white"
              @click="setFollow(true)"
            >
              <ArrowDownToLine class="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent>滚动到底部</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger as-child>
            <button
              type="button"
              class="rounded p-1 text-zinc-400 transition-colors hover:bg-white/10 hover:text-white"
              @click="copyText(text, '日志已复制')"
            >
              <Copy class="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent>复制全部</TooltipContent>
        </Tooltip>
      </div>
    </div>
    <div ref="box" class="scrollbar-thin overflow-auto px-4 py-3 font-mono text-log" :style="{ height }" @scroll.passive="onScroll">
      <div v-if="!lines.length" class="text-zinc-500">{{ live ? '等待日志输出…' : '（无日志）' }}</div>
      <div v-for="(l, i) in lines" :key="i" class="break-all whitespace-pre-wrap">
        <span v-if="l.time" class="mr-2 text-zinc-500 select-none">{{ l.time }}</span><span :class="cn(l.cls)">{{ l.body }}</span>
      </div>
    </div>
    <div
      v-if="progress"
      class="flex items-center gap-2 border-t border-white/10 bg-white/[0.03] px-4 py-1.5 font-mono text-log text-amber-300"
    >
      <span class="size-1.5 shrink-0 animate-pulse rounded-full bg-amber-300" />
      <template v-if="bar">
        <span class="shrink-0 tabular-nums">{{ bar.done }} / {{ bar.total }}</span>
        <span class="h-1.5 min-w-8 flex-1 overflow-hidden rounded-full bg-amber-300/15">
          <span class="block h-full rounded-full bg-amber-300 transition-[width] duration-300" :style="{ width: `${bar.pct}%` }" />
        </span>
        <span class="shrink-0 tabular-nums">{{ Math.floor(bar.pct) }}%</span>
        <span v-if="bar.tail" class="shrink-0 text-amber-300/80">{{ bar.tail }}</span>
      </template>
      <span v-else class="min-w-0 truncate">{{ progress }}</span>
    </div>
  </div>
</template>
