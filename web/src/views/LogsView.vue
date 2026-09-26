<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RefreshCw, Search, Table2, Terminal } from '@lucide/vue'
import { systemApi } from '@/api'
import type { LogLine } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import LogPanel from '@/components/LogPanel.vue'
import PageHeader from '@/components/PageHeader.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import { fmtTime, type Tone } from '@/utils/format'

const { t } = useI18n()
const lines = ref<LogLine[]>([])
const loading = ref(false)
const level = ref('ALL')
const keyword = ref('')
const limit = ref('500')
const auto = ref(false)
const view = ref<'terminal' | 'table'>('terminal')

let timer: ReturnType<typeof setInterval> | undefined

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const r = await systemApi.logs(Number(limit.value), silent)
    lines.value = r.lines ?? []
  } catch {
    /* 已提示 */
  } finally {
    loading.value = false
  }
}

const LEVELS = ['DEBUG', 'INFO', 'WARN', 'ERROR']
const levelRank = (l: string) => {
  const i = LEVELS.indexOf((l || '').toUpperCase().replace('WARNING', 'WARN'))
  return i < 0 ? 1 : i
}

const filtered = computed(() => {
  const min = level.value !== 'ALL' ? levelRank(level.value) : -1
  const kw = keyword.value.trim().toLowerCase()
  return lines.value.filter((l) => {
    if (min >= 0 && levelRank(l.level) < min) return false
    if (kw && !l.message.toLowerCase().includes(kw)) return false
    return true
  })
})

// 终端视图：按级别加上 ✗ / ! 前缀以便着色
const text = computed(() =>
  filtered.value
    .map((l) => {
      const r = levelRank(l.level)
      const mark = r === 3 ? '✗ ' : r === 2 ? '! ' : ''
      return `${fmtTime(l.time)} ${mark}${(l.level || '').toUpperCase().padEnd(5)} ${l.message}`
    })
    .join('\n'),
)

function levelTone(l: string): Tone {
  return (['neutral', 'info', 'warning', 'danger'] as Tone[])[levelRank(l)] ?? 'info'
}

watch(auto, (v) => {
  clearInterval(timer)
  if (v) {
    timer = setInterval(() => {
      if (document.visibilityState === 'visible') load(true)
    }, 3000)
  }
})

onMounted(() => load())
onBeforeUnmount(() => clearInterval(timer))

function setView(v: unknown) {
  view.value = v === 'table' ? 'table' : 'terminal'
}
function setLimit(v: unknown) {
  limit.value = String(v)
  load()
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader :title="t('logs.title')" :description="t('logs.description')">
      <Tabs :model-value="view" @update:model-value="setView">
        <TabsList class="h-8">
          <TabsTrigger value="terminal" class="text-xs"><Terminal class="size-3.5" />{{ t('logs.terminal') }}</TabsTrigger>
          <TabsTrigger value="table" class="text-xs"><Table2 class="size-3.5" />{{ t('logs.table') }}</TabsTrigger>
        </TabsList>
      </Tabs>
      <Button variant="outline" size="sm" :disabled="loading" @click="load()"><RefreshCw :class="loading ? 'animate-spin' : ''" />{{ t('common.refresh') }}</Button>
    </PageHeader>

    <div class="flex flex-wrap items-center gap-2">
      <Select v-model="level">
        <SelectTrigger class="w-36"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem value="ALL">{{ t('logs.allLevels') }}</SelectItem>
          <SelectItem value="DEBUG">{{ t('logs.atLeast', { level: 'DEBUG' }) }}</SelectItem>
          <SelectItem value="INFO">{{ t('logs.atLeast', { level: 'INFO' }) }}</SelectItem>
          <SelectItem value="WARN">{{ t('logs.atLeast', { level: 'WARN' }) }}</SelectItem>
          <SelectItem value="ERROR">{{ t('logs.only', { level: 'ERROR' }) }}</SelectItem>
        </SelectContent>
      </Select>
      <div class="relative">
        <Search class="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
        <Input v-model="keyword" :placeholder="t('logs.keyword')" class="w-56 pl-8" />
      </div>
      <Select :model-value="limit" @update:model-value="setLimit">
        <SelectTrigger class="w-32"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem v-for="n in ['200', '500', '1000', '2000']" :key="n" :value="n">{{ t('logs.recent', { n }) }}</SelectItem>
        </SelectContent>
      </Select>
      <label class="flex cursor-pointer items-center gap-2 text-sm">
        <Switch v-model="auto" />{{ t('logs.autoRefresh') }}
      </label>
      <span class="text-muted-foreground ml-auto text-xs tabular-nums">{{ t('logs.count', { shown: filtered.length, total: lines.length }) }}</span>
    </div>

    <LogPanel v-if="view === 'terminal'" :text="text" :live="auto" title="cfst-ddns · app.log" height="calc(100vh - 290px)" />
    <Card v-else class="gap-0 overflow-hidden py-0">
      <div class="overflow-auto" style="max-height: calc(100vh - 290px)">
        <Table>
          <TableHeader class="bg-card sticky top-0">
            <TableRow>
              <TableHead class="w-44 pl-5">{{ t('logs.time') }}</TableHead>
              <TableHead class="w-24">{{ t('logs.level') }}</TableHead>
              <TableHead class="pr-5">{{ t('logs.message') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="(l, i) in filtered" :key="i">
              <TableCell class="pl-5 font-mono text-code tabular-nums">{{ fmtTime(l.time) }}</TableCell>
              <TableCell><ToneBadge :tone="levelTone(l.level)">{{ (l.level || '').toUpperCase() }}</ToneBadge></TableCell>
              <TableCell class="pr-5 font-mono text-code break-all whitespace-pre-wrap">{{ l.message }}</TableCell>
            </TableRow>
            <TableRow v-if="!filtered.length">
              <TableCell colspan="3" class="text-muted-foreground py-10 text-center">{{ t('logs.empty') }}</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </Card>
  </div>
</template>
