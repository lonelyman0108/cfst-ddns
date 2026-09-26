<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { ChevronDown, CircleAlert, Loader2 } from '@lucide/vue'
import { cronApi, errorMessage } from '@/api'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectSeparator, SelectTrigger, SelectValue } from '@/components/ui/select'
import { CRON_PRESETS, describeCron } from '@/utils/cron'
import { fmtTime, fromNow } from '@/utils/format'

const model = defineModel<string>({ default: '' })
const emit = defineEmits<{ (e: 'next', v: string[]): void }>()

const CUSTOM = '__custom__'
// Select 不接受空字符串作为选项值，“仅手动”使用占位键
const MANUAL = '__manual__'
const keyOf = (v: string) => (v === '' ? MANUAL : v)
const presetOf = (v: string) => (CRON_PRESETS.some((p) => p.value === v.trim()) ? keyOf(v.trim()) : CUSTOM)

const mode = ref(presetOf(model.value))
const custom = ref(mode.value === CUSTOM ? model.value : '')
const next = ref<string[]>([])
const error = ref('')
const loading = ref(false)
const showMore = ref(false)

let timer: ReturnType<typeof setTimeout> | undefined
let seq = 0

function preview(expr: string) {
  clearTimeout(timer)
  error.value = ''
  next.value = []
  emit('next', [])
  const e = expr.trim()
  if (!e) {
    loading.value = false
    return
  }
  loading.value = true
  timer = setTimeout(async () => {
    const my = ++seq
    try {
      const r = await cronApi.preview(e)
      if (my === seq) {
        next.value = r.next ?? []
        emit('next', next.value)
      }
    } catch (err) {
      if (my === seq) error.value = errorMessage(err)
    } finally {
      if (my === seq) loading.value = false
    }
  }, 400)
}

function onModeChange(v: unknown) {
  const s = String(v)
  mode.value = s
  if (s === CUSTOM) {
    custom.value = custom.value || model.value
    model.value = custom.value
  } else {
    model.value = s === MANUAL ? '' : s
  }
}

function onCustomInput(v: string | number) {
  custom.value = String(v)
  model.value = custom.value
}

watch(
  model,
  (v) => {
    // 外部赋值（如加载任务）时同步下拉状态
    const p = presetOf(v ?? '')
    if (p !== CUSTOM && mode.value !== CUSTOM) mode.value = p
    else if (p === CUSTOM) {
      mode.value = CUSTOM
      custom.value = v
    }
    preview(v ?? '')
  },
  { immediate: true },
)

onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <div class="grid grid-cols-1 gap-1.5">
    <div class="flex flex-wrap gap-2">
      <Select :model-value="mode" @update:model-value="onModeChange">
        <SelectTrigger class="w-36">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="p in CRON_PRESETS" :key="keyOf(p.value)" :value="keyOf(p.value)">{{ p.label }}</SelectItem>
          <SelectSeparator />
          <SelectItem :value="CUSTOM">自定义 cron</SelectItem>
        </SelectContent>
      </Select>
      <Input
        v-if="mode === CUSTOM"
        :model-value="custom"
        class="min-w-48 flex-1 font-mono"
        placeholder="分 时 日 月 周，如 0 */2 * * *"
        @update:model-value="onCustomInput"
      />
    </div>
    <!-- 一行摘要：表达式 · 描述 · 下次执行；可展开查看未来 5 次 -->
    <div v-if="!model.trim()" class="text-muted-foreground text-xs">不自动执行，仅可手动或通过 Webhook 触发</div>
    <div v-else-if="error" class="text-destructive flex items-center gap-1.5 text-xs">
      <CircleAlert class="size-3.5 shrink-0" /><span class="font-mono">{{ model }}</span> · {{ error }}
    </div>
    <div v-else class="text-muted-foreground flex flex-wrap items-center gap-x-1.5 text-xs">
      <span class="text-foreground font-mono">{{ model.trim() }}</span>
      <template v-if="describeCron(model) !== model.trim()"><span>·</span><span>{{ describeCron(model) }}</span></template>
      <span>·</span>
      <Loader2 v-if="loading" class="size-3.5 animate-spin" />
      <template v-else-if="next.length">
        <span>下次 <span class="text-foreground tabular-nums">{{ fmtTime(next[0], 'MM-DD HH:mm') }}</span>（{{ fromNow(next[0]) }}）</span>
        <button type="button" class="text-primary ml-1 inline-flex items-center hover:underline" @click="showMore = !showMore">
          {{ showMore ? '收起' : '查看更多' }}<ChevronDown :class="['size-3.5 transition-transform', showMore && 'rotate-180']" />
        </button>
      </template>
    </div>
    <div v-if="showMore && next.length && !error && model.trim()" class="bg-muted/40 grid grid-cols-1 gap-0.5 rounded-md border px-3 py-1.5">
      <div v-for="t in next" :key="t" class="flex items-center justify-between gap-4 text-xs">
        <span class="font-mono">{{ fmtTime(t) }}</span>
        <span class="text-muted-foreground">{{ fromNow(t) }}</span>
      </div>
    </div>
  </div>
</template>
