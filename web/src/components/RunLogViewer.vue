<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { useI18n } from 'vue-i18n'
import { ChevronDown, CircleX, Gauge, ListTree, Network, RefreshCw, ScrollText, Timer, TriangleAlert, Zap } from '@lucide/vue'
import { runsApi, tasksApi } from '@/api'
import type { NetworkProbe, RunDetail, RunSummary, SpeedResult } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { confirm } from '@/composables/useConfirm'
import StatusBadge from './StatusBadge.vue'
import ToneBadge from './ToneBadge.vue'
import CopyText from './CopyText.vue'
import LogPanel from './LogPanel.vue'
import EmptyState from './EmptyState.vue'
import {
  changeActionMeta,
  changeMessage,
  dayjs,
  fmtDuration,
  fmtLatency,
  fmtPercent,
  fmtSpeed,
  fmtTime,
  isRunActive,
  runMessage,
  runTriggerLabel,
} from '@/utils/format'

const props = withDefaults(defineProps<{ runId: number; logHeight?: string }>(), { logHeight: '440px' })
const emit = defineEmits<{
  (e: 'status', s: RunSummary): void
  (e: 'done', s: RunSummary): void
  /** 切换为经代理模式后重新执行，参数为新的 run id */
  (e: 'rerun', runId: number): void
}>()

const { t } = useI18n()
const summary = ref<RunSummary | null>(null)
const detail = ref<RunDetail | null>(null)
const loading = ref(false)
const loadError = ref('')
const lines = ref<string[]>([])
const progress = ref('')
const streaming = ref(false)
const canceling = ref(false)
const now = ref(Date.now())

let es: EventSource | null = null
let retryTimer: ReturnType<typeof setTimeout> | undefined
let tick: ReturnType<typeof setInterval> | undefined
let pending: string[] = []
let raf = 0
let disposed = false

const active = computed(() => isRunActive(summary.value?.status))
const logText = computed(() =>
  streaming.value || !detail.value ? lines.value.join('\n') : detail.value.log || lines.value.join('\n'),
)

const duration = computed(() => {
  const s = summary.value
  if (!s) return '-'
  if (active.value && s.startedAt) return fmtDuration(now.value - dayjs(s.startedAt).valueOf())
  return fmtDuration(s.durationMs)
})

/** 测速结果默认只显示前几名，展开后显示全部已保存的结果 */
const PREVIEW_ROWS = 10
const expanded = ref<Record<string, boolean>>({})

const groups = computed(() => {
  const res = detail.value?.results ?? []
  const out: { type: 'v4' | 'v6'; label: string; rows: SpeedResult[]; stored: number; total: number }[] = []
  for (const ty of ['v4', 'v6'] as const) {
    const rows = res.filter((r) => r.ipType === ty).sort((a, b) => a.rank - b.rank)
    if (!rows.length) continue
    out.push({
      type: ty,
      label: ty === 'v4' ? 'IPv4' : 'IPv6',
      rows: expanded.value[ty] ? rows : rows.slice(0, PREVIEW_ROWS),
      stored: rows.length,
      total: detail.value?.resultTotals?.[ty] ?? rows.length,
    })
  }
  return out
})

const network = computed(() => detail.value?.network ?? null)

/** 自检状态：拦截 > 未连通 > 出口不一致（仅直连模式提示）> 正常 */
function probeState(p: NetworkProbe): { tone: 'danger' | 'warning' | 'neutral' | 'success'; label: string } {
  if (p.intercepted) return { tone: 'danger', label: t('runs.network.intercepted') }
  if (!p.egressIP && !p.connectMs) return { tone: 'neutral', label: t('runs.network.unreachable') }
  if (network.value?.mode !== 'proxy' && p.egressIP && p.localIP && p.egressIP !== p.localIP)
    return { tone: 'warning', label: t('runs.network.mismatch') }
  return { tone: 'success', label: t('runs.network.ok') }
}

const switching = ref(false)

/** 用户确认是有意经代理测速：把任务改为经代理模式（强制 HTTPing）并重新执行 */
async function switchToProxy() {
  const s = summary.value
  if (!s) return
  const ok = await confirm({
    title: t('runs.network.switchTitle'),
    description: t('runs.network.switchDesc'),
    confirmText: t('runs.network.switchConfirm'),
  })
  if (!ok) return
  switching.value = true
  try {
    const task = await tasksApi.get(s.taskId)
    const { name, enabled, cron, ipType, speedTest, update, targets, notifierIds } = task
    await tasksApi.update(task.id, {
      name, enabled, cron, ipType, update, targets, notifierIds,
      speedTest: { ...speedTest, networkMode: 'proxy', httping: true },
    })
    const { runId } = await tasksApi.run(task.id, { dryRun: s.dryRun })
    toast.success(t('runs.network.switched'))
    emit('rerun', runId)
  } catch {
    /* 已提示 */
  } finally {
    switching.value = false
  }
}

function setSummary(s: RunSummary) {
  summary.value = s
  emit('status', s)
}

function flush() {
  raf = 0
  if (!pending.length) return
  lines.value.push(...pending)
  pending = []
}

function pushLine(l: string) {
  pending.push(l)
  if (!raf) raf = requestAnimationFrame(flush)
}

function closeStream() {
  if (es) {
    es.close()
    es = null
  }
  streaming.value = false
  clearTimeout(retryTimer)
}

async function loadDetail() {
  loading.value = true
  loadError.value = ''
  try {
    const d = await runsApi.get(props.runId)
    detail.value = d
    setSummary(d)
    return d
  } catch (e) {
    loadError.value = 'failed'
    throw e
  } finally {
    loading.value = false
  }
}

function parse(data: string): RunSummary | null {
  try {
    return JSON.parse(data) as RunSummary
  } catch {
    return null
  }
}

function openStream() {
  closeStream()
  if (disposed) return
  lines.value = []
  pending = []
  progress.value = ''
  streaming.value = true
  const src = new EventSource(runsApi.streamUrl(props.runId))
  es = src
  src.addEventListener('open', () => {
    // 服务端连接时会补发已有日志，重连时先清空避免重复
    lines.value = []
    pending = []
  })
  src.addEventListener('log', (ev) => pushLine((ev as MessageEvent<string>).data))
  src.addEventListener('progress', (ev) => {
    progress.value = (ev as MessageEvent<string>).data
  })
  src.addEventListener('status', (ev) => {
    const s = parse((ev as MessageEvent<string>).data)
    if (s) setSummary(s)
  })
  src.addEventListener('done', async (ev) => {
    const s = parse((ev as MessageEvent<string>).data)
    flush()
    closeStream()
    progress.value = ''
    if (s) {
      setSummary(s)
      emit('done', s)
    }
    await loadDetail().catch(() => {})
  })
  src.onerror = () => {
    if (es !== src) return
    // 连接断开：检查执行是否已结束，仍在运行则稍后重连
    src.close()
    es = null
    retryTimer = setTimeout(async () => {
      try {
        const d = await loadDetail()
        if (isRunActive(d.status)) openStream()
        else {
          streaming.value = false
          progress.value = ''
          emit('done', d)
        }
      } catch {
        streaming.value = false
      }
    }, 1500)
  }
}

async function start() {
  closeStream()
  detail.value = null
  summary.value = null
  lines.value = []
  progress.value = ''
  try {
    const d = await loadDetail()
    if (isRunActive(d.status)) openStream()
  } catch {
    /* 已提示 */
  }
}

async function cancel() {
  const ok = await confirm({
    title: t('runs.viewer.cancel'),
    description: t('runs.viewer.cancelDesc'),
    confirmText: t('runs.viewer.cancel'),
    cancelText: t('runs.viewer.keepRunning'),
    destructive: true,
  })
  if (!ok) return
  canceling.value = true
  try {
    await runsApi.cancel(props.runId)
    toast.success(t('runs.viewer.cancelSent'))
  } catch {
    /* 已提示 */
  } finally {
    canceling.value = false
  }
}

watch(() => props.runId, start)

onMounted(() => {
  start()
  tick = setInterval(() => (now.value = Date.now()), 1000)
})

onBeforeUnmount(() => {
  disposed = true
  closeStream()
  clearInterval(tick)
  if (raf) cancelAnimationFrame(raf)
})

defineExpose({ reload: start })
</script>

<template>
  <div class="flex flex-col gap-4">
    <div v-if="loading && !summary" class="grid grid-cols-1 gap-4">
      <Skeleton class="h-36 w-full rounded-xl" />
      <Skeleton class="h-72 w-full rounded-xl" />
    </div>

    <EmptyState v-else-if="loadError && !summary" :icon="CircleX" :title="t('runs.viewer.loadFailed')" :description="t('runs.viewer.loadFailedDesc')">
      <Button variant="outline" size="sm" @click="start"><RefreshCw />{{ t('common.retry') }}</Button>
    </EmptyState>

    <template v-if="summary">
      <Card>
        <CardContent class="grid grid-cols-1 gap-3">
          <div class="flex flex-wrap items-center gap-2">
            <StatusBadge :status="summary.status" />
            <ToneBadge v-if="summary.dryRun" tone="info" :title="t('runs.dryRunTip')">{{ t('runs.dryRun') }}</ToneBadge>
            <router-link :to="`/tasks/${summary.taskId}`" class="font-semibold hover:underline">
              {{ summary.taskName || t('runs.viewer.taskFallback', { id: summary.taskId }) }}
            </router-link>
            <span class="text-muted-foreground text-xs">#{{ summary.id }} · {{ t('runs.viewer.triggered', { trigger: runTriggerLabel[summary.trigger] ?? summary.trigger }) }}</span>
            <div class="ml-auto flex gap-2">
              <Button v-if="active" variant="destructive" size="sm" :disabled="canceling" @click="cancel">
                <CircleX />{{ t('runs.viewer.cancel') }}
              </Button>
              <Button v-else variant="outline" size="sm" @click="start"><RefreshCw />{{ t('common.refresh') }}</Button>
            </div>
          </div>
          <div class="grid grid-cols-2 gap-x-6 gap-y-3 text-sm md:grid-cols-4">
            <div>
              <div class="text-muted-foreground flex items-center gap-1 text-xs"><Timer class="size-3.5" />{{ t('runs.viewer.started') }}</div>
              <div class="mt-0.5 font-medium tabular-nums">{{ fmtTime(summary.startedAt) }}</div>
            </div>
            <div>
              <div class="text-muted-foreground text-xs">{{ t('runs.duration') }}</div>
              <div class="mt-0.5 font-medium tabular-nums">{{ duration }}</div>
            </div>
            <div>
              <div class="text-muted-foreground flex items-center gap-1 text-xs"><Gauge class="size-3.5" />{{ t('runs.viewer.minLatency') }}</div>
              <div class="mt-0.5 font-medium tabular-nums">{{ fmtLatency(summary.bestLatency) }}</div>
            </div>
            <div>
              <div class="text-muted-foreground flex items-center gap-1 text-xs"><Zap class="size-3.5" />{{ t('runs.viewer.maxSpeed') }}</div>
              <div class="mt-0.5 font-medium tabular-nums">{{ fmtSpeed(summary.bestSpeed) }}</div>
            </div>
            <div class="min-w-0">
              <div class="text-muted-foreground text-xs">{{ t('runs.viewer.bestV4') }}</div>
              <div class="mt-0.5"><CopyText v-if="summary.bestIPv4" :text="summary.bestIPv4" /><span v-else>-</span></div>
            </div>
            <div class="min-w-0">
              <div class="text-muted-foreground text-xs">{{ t('runs.viewer.bestV6') }}</div>
              <div class="mt-0.5"><CopyText v-if="summary.bestIPv6" :text="summary.bestIPv6" /><span v-else>-</span></div>
            </div>
            <div>
              <div class="text-muted-foreground text-xs">{{ t('runs.viewer.dnsChanges') }}</div>
              <div class="mt-1">
                <ToneBadge v-if="!active" :tone="summary.changed ? 'success' : 'neutral'">{{ summary.changed ? t('runs.viewer.changed') : t('runs.viewer.unchanged') }}</ToneBadge>
                <span v-else>-</span>
              </div>
            </div>
            <div>
              <div class="text-muted-foreground text-xs">{{ t('runs.viewer.finished') }}</div>
              <div class="mt-0.5 font-medium tabular-nums">{{ fmtTime(summary.finishedAt) }}</div>
            </div>
          </div>
          <Alert
            v-if="summary.message"
            :variant="summary.status === 'failed' ? 'destructive' : 'default'"
            :class="summary.status === 'partial' ? 'border-warning/40 text-warning' : ''"
          >
            <AlertDescription :class="summary.status === 'failed' ? 'text-destructive' : ''">{{ runMessage(summary) }}</AlertDescription>
          </Alert>
        </CardContent>
      </Card>

      <template v-if="!active && detail">
        <Card v-if="network" class="gap-0 py-0">
          <CardHeader class="border-b py-3 [.border-b]:pb-3">
            <CardTitle class="flex items-center gap-2">
              <Network class="text-muted-foreground size-4" />{{ t('runs.network.title') }}
              <ToneBadge :tone="network.mode === 'proxy' ? 'info' : 'neutral'">{{ network.mode === 'proxy' ? t('runs.network.modeProxy') : t('runs.network.modeDirect') }}</ToneBadge>
            </CardTitle>
          </CardHeader>
          <CardContent class="p-0">
            <Alert v-if="network.proxySuspected" variant="destructive" class="m-4 w-auto">
              <TriangleAlert />
              <AlertTitle class="line-clamp-none">{{ t('runs.network.suspectedTitle') }}</AlertTitle>
              <AlertDescription class="grid grid-cols-1 gap-3">
                <p>{{ network.reason === 'latency' ? t('runs.network.reasonLatency') : t('runs.network.reasonIntercepted') }} {{ t('runs.network.suspectedHint') }}</p>
                <div class="flex flex-wrap gap-2">
                  <Button size="sm" variant="outline" class="h-auto min-h-8 max-w-full shrink py-1.5 text-left whitespace-normal" :disabled="switching" @click="switchToProxy">{{ t('runs.network.intentional') }}</Button>
                  <Button size="sm" variant="ghost" as-child>
                    <router-link to="/help#faq">{{ t('runs.network.howToFix') }}</router-link>
                  </Button>
                </div>
              </AlertDescription>
            </Alert>
            <Table v-if="network.probes?.length">
              <TableHeader>
                <TableRow>
                  <TableHead class="w-16 pl-5">{{ t('common.type') }}</TableHead>
                  <TableHead>{{ t('runs.network.colState') }}</TableHead>
                  <TableHead class="hidden text-right sm:table-cell">{{ t('runs.network.colConnect') }}</TableHead>
                  <TableHead>{{ t('runs.network.colEgress') }}</TableHead>
                  <TableHead class="hidden md:table-cell">{{ t('runs.network.colLoc') }}</TableHead>
                  <TableHead class="hidden pr-5 sm:table-cell">{{ t('runs.network.colLocal') }}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="p in network.probes" :key="p.ipType">
                  <TableCell class="pl-5 font-mono text-code">{{ p.ipType === 'v6' ? 'IPv6' : 'IPv4' }}</TableCell>
                  <TableCell><ToneBadge :tone="probeState(p).tone" :title="p.error">{{ probeState(p).label }}</ToneBadge></TableCell>
                  <TableCell class="hidden text-right tabular-nums sm:table-cell">{{ p.connectMs ? fmtLatency(p.connectMs) : '-' }}</TableCell>
                  <TableCell class="min-w-28 pr-5 whitespace-normal break-all sm:pr-2"><CopyText v-if="p.egressIP" :text="p.egressIP" /><span v-else>-</span></TableCell>
                  <TableCell class="hidden font-mono text-code md:table-cell">{{ [p.loc, p.colo].filter(Boolean).join(' · ') || '-' }}</TableCell>
                  <TableCell class="hidden min-w-36 pr-5 whitespace-normal break-all sm:table-cell"><CopyText v-if="p.localIP" :text="p.localIP" /><span v-else>-</span></TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card class="gap-0 py-0">
          <CardHeader class="border-b py-3 [.border-b]:pb-3">
            <CardTitle class="flex items-center gap-2"><Gauge class="text-muted-foreground size-4" />{{ t('runs.viewer.results') }}</CardTitle>
          </CardHeader>
          <CardContent class="p-0">
            <EmptyState v-if="!groups.length" compact :title="t('runs.viewer.noResults')" />
            <div v-for="g in groups" :key="g.type" class="border-b last:border-b-0">
              <div class="text-muted-foreground bg-muted/30 px-5 py-2 text-xs font-medium">
                {{ t('runs.viewer.groupTotal', { label: g.label, total: g.total }) }}<template v-if="g.total > g.stored">{{ t('runs.viewer.groupStored', { n: g.stored }) }}</template>
              </div>
              <div :class="expanded[g.type] && 'scrollbar-thin max-h-[560px] overflow-y-auto'">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead class="w-12 pl-5">#</TableHead>
                      <TableHead>IP</TableHead>
                      <TableHead class="text-right">{{ t('runs.viewer.colLatency') }}</TableHead>
                      <TableHead class="text-right">{{ t('runs.viewer.colSpeed') }}</TableHead>
                      <TableHead class="text-right">{{ t('runs.viewer.colLoss') }}</TableHead>
                      <TableHead class="text-right">{{ t('runs.viewer.colSentRecv') }}</TableHead>
                      <TableHead class="pr-5 text-right">{{ t('runs.viewer.colColo') }}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow v-for="r in g.rows" :key="r.ip">
                      <TableCell class="text-muted-foreground pl-5 tabular-nums">{{ r.rank }}</TableCell>
                      <TableCell><CopyText :text="r.ip" /></TableCell>
                      <TableCell class="text-right tabular-nums">{{ fmtLatency(r.latency) }}</TableCell>
                      <TableCell class="text-right tabular-nums">{{ fmtSpeed(r.speed) }}</TableCell>
                      <TableCell class="text-right tabular-nums">{{ fmtPercent(r.lossRate) }}</TableCell>
                      <TableCell class="text-right tabular-nums">{{ r.sent }} / {{ r.received }}</TableCell>
                      <TableCell class="pr-5 text-right font-mono text-code">{{ r.colo || '-' }}</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </div>
              <div v-if="g.stored > PREVIEW_ROWS" class="border-t px-5 py-2">
                <Button variant="ghost" size="sm" class="text-muted-foreground -ml-2" @click="expanded[g.type] = !expanded[g.type]">
                  <ChevronDown :class="['transition-transform', expanded[g.type] && 'rotate-180']" />
                  {{ expanded[g.type] ? t('common.collapse') : t('common.showAll', { n: g.stored }) }}
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card class="gap-0 py-0">
          <CardHeader class="border-b py-3 [.border-b]:pb-3">
            <CardTitle class="flex items-center gap-2"><ListTree class="text-muted-foreground size-4" />{{ t('runs.viewer.dnsChanges') }}</CardTitle>
          </CardHeader>
          <CardContent class="p-0">
            <EmptyState v-if="!detail.changes?.length" compact :title="t('runs.viewer.noChanges')" />
            <Table v-else>
              <TableHeader>
                <TableRow>
                  <TableHead class="w-20 pl-5">{{ t('common.actions') }}</TableHead>
                  <TableHead>{{ t('runs.viewer.colRecord') }}</TableHead>
                  <TableHead class="w-16">{{ t('common.type') }}</TableHead>
                  <TableHead>{{ t('runs.viewer.colValues') }}</TableHead>
                  <TableHead class="hidden xl:table-cell">{{ t('runs.viewer.colAccount') }}</TableHead>
                  <TableHead class="pr-5">{{ t('runs.message') }}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="(c, i) in detail.changes" :key="i">
                  <TableCell class="pl-5">
                    <ToneBadge :tone="changeActionMeta[c.action]?.type ?? 'neutral'">{{ changeActionMeta[c.action]?.label ?? c.action }}</ToneBadge>
                  </TableCell>
                  <TableCell class="min-w-48 font-medium whitespace-normal break-all">{{ c.fqdn }}</TableCell>
                  <TableCell class="font-mono text-code">{{ c.type }}</TableCell>
                  <TableCell class="text-code min-w-40 font-mono whitespace-normal break-all">
                    <div class="text-muted-foreground">{{ c.oldValue || '∅' }}</div>
                    <div><span class="text-muted-foreground mr-1">→</span>{{ c.newValue || '∅' }}</div>
                  </TableCell>
                  <TableCell class="text-muted-foreground hidden xl:table-cell min-w-32 whitespace-normal break-words">{{ c.accountName }}</TableCell>
                  <TableCell class="text-muted-foreground max-w-80 min-w-40 pr-5 text-xs whitespace-normal break-words">{{ changeMessage(c) || '-' }}</TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </template>

      <div>
        <div class="mb-2 flex items-center gap-2 text-sm font-medium">
          <ScrollText class="text-muted-foreground size-4" />{{ active ? t('runs.viewer.liveLog') : t('runs.viewer.fullLog') }}
        </div>
        <LogPanel
          :text="logText"
          :progress="streaming ? progress : ''"
          :live="streaming"
          :title="`run #${summary.id} · ${summary.taskName}`"
          :height="logHeight"
        />
      </div>
    </template>
  </div>
</template>
