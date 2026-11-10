<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleX, ExternalLink, Gauge, RefreshCw, TriangleAlert } from '@lucide/vue'
import { tasksApi } from '@/api'
import type { HealthCheckHistory, HealthCheckStatus, HealthResult, HealthStatus } from '@/api/types'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import EmptyState from '@/components/EmptyState.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import { useVisibleInterval } from '@/utils/useMedia'
import { fmtLatency, fmtPercent, fmtTime, fromNow, dayjs } from '@/utils/format'

const props = defineProps<{
  taskId: number
  enabled?: boolean
  initialState?: HealthStatus
  nextRunAt?: string | null
}>()

const { t } = useI18n()
const loading = ref(false)
const loaded = ref(false)
const failed = ref(false)
const items = ref<HealthCheckHistory[]>([])
const state = ref<HealthStatus | undefined>(props.initialState)

const latest = computed(() => items.value[0])
const effectiveState = computed(() => state.value ?? props.initialState)

const statusLabel = (status?: HealthCheckStatus | null) => {
  if (status === 'healthy') return t('health.history.statusHealthy')
  if (status === 'unhealthy') return t('health.history.statusUnhealthy')
  if (status === 'unknown') return t('health.history.statusUnknown')
  if (status === 'skipped') return t('health.history.statusSkipped')
  return t('health.history.statusUnknown')
}

const statusTone = (status?: HealthCheckStatus | null) => {
  if (status === 'healthy') return 'success' as const
  if (status === 'unhealthy') return 'danger' as const
  if (status === 'unknown') return 'warning' as const
  if (status === 'skipped') return 'neutral' as const
  return 'neutral' as const
}

function resultStatus(result: HealthResult, item: HealthCheckHistory): HealthCheckStatus {
  if (result.status) return result.status
  if (item.status === 'unknown' || item.status === 'skipped') return item.status
  if (result.reason?.toLowerCase().includes('unknown') || result.reason?.toLowerCase().includes('provider')) return 'unknown'
  return result.healthy ? 'healthy' : 'unhealthy'
}

function reasonLabel(reason?: string) {
  if (!reason) return ''
  const known: Record<string, string> = {
    proxied: 'health.history.reasonProxySkipped',
    proxySkipped: 'health.history.reasonProxySkipped',
    provider: 'health.history.reasonProviderUnavailable',
    providerUnavailable: 'health.history.reasonProviderUnavailable',
    unsupportedLine: 'health.history.reasonUnsupportedLine',
    missingResult: 'health.history.reasonNoResult',
    noResult: 'health.history.reasonNoResult',
    probeError: 'health.history.reasonProbeError',
    invalidIP: 'health.history.reasonInvalidIp',
    proxyDetected: 'health.history.reasonProxyDetected',
    noRecords: 'health.history.reasonNoRecords',
    busy: 'health.history.reasonBusy',
    disabled: 'health.history.reasonDisabled',
    unreachable: 'health.history.reasonUnreachable',
    timeout: 'health.history.reasonTimeout',
    unknown: 'health.history.statusUnknown',
    skipped: 'health.history.statusSkipped',
    latency: 'health.history.reasonThresholdLatency',
    speed: 'health.history.reasonThresholdSpeed',
    loss: 'health.history.reasonLoss',
  }
  return known[reason] ? t(known[reason]) : reason
}

function targetLabel(result: HealthResult) {
  if (result.fqdn) return result.accountName ? `${result.fqdn} · ${result.accountName}` : result.fqdn
  if (result.accountName) return result.accountName
  return t('health.history.targetFallback', { key: result.targetKey })
}

function metric(value: number | null | undefined, measured = true, formatter: (v: number) => string) {
  return measured && typeof value === 'number' && Number.isFinite(value) && value > 0 ? formatter(value) : t('health.history.measuredUnavailable')
}

function speedMetric(result: HealthResult) {
  if (result.speedMeasured !== false && typeof result.speed === 'number' && Number.isFinite(result.speed) && result.speed >= 0) {
    return `${result.speed.toFixed(2)} MB/s`
  }
  return t('health.history.measuredUnavailable')
}

function lossMetric(result: HealthResult) {
  return result.lossMeasured !== false && typeof result.lossRate === 'number' && Number.isFinite(result.lossRate)
    ? fmtPercent(result.lossRate)
    : t('health.history.measuredUnavailable')
}

const cooldownActive = computed(() => {
  const until = effectiveState.value?.cooldownUntil
  return !!until && dayjs(until).isAfter(dayjs())
})

async function load() {
  if (!props.taskId) return
  loading.value = true
  failed.value = false
  try {
    const response = await tasksApi.health(props.taskId, 50)
    state.value = response?.state ?? props.initialState
    items.value = response?.items ?? []
    loaded.value = true
  } catch {
    failed.value = true
  } finally {
    loading.value = false
  }
}

onMounted(load)
useVisibleInterval(() => {
  if (!loading.value) load()
}, 30000)

defineExpose({ reload: load })
</script>

<template>
  <Card class="gap-0 overflow-hidden py-0">
    <CardHeader class="border-b py-4 [.border-b]:pb-4">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <CardTitle class="flex items-center gap-2"><Gauge class="text-muted-foreground size-4" />{{ t('health.history.title') }}</CardTitle>
          <CardDescription class="mt-1">{{ t('health.history.summary') }}</CardDescription>
        </div>
        <Button type="button" variant="ghost" size="icon" class="size-8" :aria-label="t('health.history.refresh')" :disabled="loading" @click="load">
          <RefreshCw :class="loading ? 'animate-spin' : ''" />
        </Button>
      </div>
    </CardHeader>
    <CardContent class="grid gap-4 p-4 sm:p-5">
      <Alert v-if="!enabled" class="border-muted-foreground/20">
        <AlertDescription>{{ t('health.history.disabled') }}</AlertDescription>
      </Alert>

      <div v-if="effectiveState || latest" class="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
        <div>
          <div class="text-muted-foreground text-xs">{{ t('health.history.latest') }}</div>
          <div class="mt-1 font-medium tabular-nums">{{ fmtTime(effectiveState?.lastCheckedAt || latest?.checkedAt) }}</div>
          <div v-if="effectiveState?.lastCheckedAt || latest?.checkedAt" class="text-muted-foreground text-xs">{{ fromNow(effectiveState?.lastCheckedAt || latest?.checkedAt) }}</div>
        </div>
        <div>
          <div class="text-muted-foreground text-xs">{{ t('health.history.status') }}</div>
          <div class="mt-1"><ToneBadge :tone="statusTone(effectiveState?.lastStatus || effectiveState?.status || latest?.status)">{{ statusLabel(effectiveState?.lastStatus || effectiveState?.status || latest?.status) }}</ToneBadge></div>
        </div>
        <div>
          <div class="text-muted-foreground text-xs">{{ t('health.history.consecutive') }}</div>
          <div class="mt-1 font-medium tabular-nums">{{ effectiveState?.consecutiveFailures ?? 0 }}</div>
        </div>
        <div>
          <div class="text-muted-foreground text-xs">{{ cooldownActive ? t('health.history.cooldown') : t('health.history.next') }}</div>
          <div class="mt-1 font-medium tabular-nums">{{ fmtTime(cooldownActive ? effectiveState?.cooldownUntil : props.nextRunAt) }}</div>
        </div>
      </div>

      <div v-if="loading && !loaded" class="grid gap-2">
        <Skeleton v-for="i in 3" :key="i" class="h-12 rounded-md" />
      </div>
      <EmptyState v-else-if="failed" :icon="CircleX" compact :title="t('health.history.loadFailed')" :description="t('health.history.loadFailedDesc')">
        <Button variant="outline" size="sm" @click="load">{{ t('common.retry') }}</Button>
      </EmptyState>
      <EmptyState v-else-if="loaded && !items.length" compact :title="t('health.history.noChecks')" :description="t('health.history.noChecksDesc')" />

      <div v-else class="grid gap-3">
        <details v-for="item in items" :key="item.id" class="group rounded-lg border" :open="item.id === latest?.id">
          <summary class="flex cursor-pointer list-none flex-wrap items-center gap-2 px-3 py-2.5 [&::-webkit-details-marker]:hidden">
            <ToneBadge :tone="statusTone(item.status)">{{ statusLabel(item.status) }}</ToneBadge>
            <span class="text-sm font-medium tabular-nums">{{ fmtTime(item.checkedAt) }}</span>
            <span class="text-muted-foreground text-xs">{{ item.results?.length ?? 0 }} IP</span>
            <span v-if="item.failureReason" class="text-muted-foreground min-w-0 flex-1 truncate text-xs" :title="reasonLabel(item.failureReason)">{{ reasonLabel(item.failureReason) }}</span>
            <router-link v-if="item.triggeredRunId" :to="`/runs/${item.triggeredRunId}`" class="text-primary ml-auto inline-flex shrink-0 items-center gap-1 text-xs hover:underline" @click.stop>
              {{ t('health.history.viewRun') }} <ExternalLink class="size-3" />
            </router-link>
          </summary>
          <div class="border-t">
            <div v-if="item.failureReason" class="text-muted-foreground flex items-start gap-1.5 px-3 py-2 text-xs">
              <TriangleAlert class="text-warning mt-0.5 size-3.5 shrink-0" />{{ reasonLabel(item.failureReason) }}
            </div>
            <div class="overflow-x-auto">
              <Table class="min-w-[680px]">
                <TableHeader>
                  <TableRow>
                    <TableHead class="pl-3">{{ t('health.history.target') }}</TableHead>
                    <TableHead>{{ t('health.history.ipType') }}</TableHead>
                    <TableHead>IP</TableHead>
                    <TableHead class="text-right">{{ t('health.history.latency') }}</TableHead>
                    <TableHead class="text-right">{{ t('health.history.speed') }}</TableHead>
                    <TableHead class="text-right">{{ t('health.history.loss') }}</TableHead>
                    <TableHead class="pr-3">{{ t('health.history.reason') }}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow v-for="(result, index) in item.results ?? []" :key="`${result.ip}-${index}`">
                    <TableCell class="max-w-56 whitespace-normal break-words pl-3 text-xs font-medium">{{ targetLabel(result) }}</TableCell>
                    <TableCell class="font-mono text-xs">{{ result.ipType }}</TableCell>
                    <TableCell class="font-mono text-xs whitespace-normal break-all">{{ result.ip || '-' }}</TableCell>
                    <TableCell class="text-right text-xs tabular-nums">{{ metric(result.latency, result.latencyMeasured !== false, fmtLatency) }}</TableCell>
                    <TableCell class="text-right text-xs tabular-nums">{{ speedMetric(result) }}</TableCell>
                    <TableCell class="text-right text-xs tabular-nums">{{ lossMetric(result) }}</TableCell>
                    <TableCell class="pr-3 text-xs whitespace-normal break-words">
                      <ToneBadge :tone="statusTone(resultStatus(result, item))">{{ statusLabel(resultStatus(result, item)) }}</ToneBadge>
                      <span v-if="result.reason" class="text-muted-foreground mt-1 block">{{ reasonLabel(result.reason) }}</span>
                      <span v-if="result.detail" class="text-muted-foreground mt-1 block break-words">{{ result.detail }}</span>
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>
            <div v-if="item.triggeredRunId" class="flex flex-wrap items-center gap-2 border-t px-3 py-2 text-xs">
              <span class="text-muted-foreground">{{ t('health.history.triggeredRun') }}:</span>
              <router-link :to="`/runs/${item.triggeredRunId}`" class="text-primary font-mono hover:underline">#{{ item.triggeredRunId }}</router-link>
            </div>
          </div>
        </details>
      </div>
    </CardContent>
  </Card>
</template>
