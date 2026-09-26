<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  Activity,
  ArrowRight,
  Bell,
  CalendarClock,
  CloudDownload,
  Globe,
  History,
  ListChecks,
  RefreshCw,
  TrendingUp,
  TriangleAlert,
  UserRoundKey,
} from '@lucide/vue'
import { dashboardApi } from '@/api'
import type { Dashboard, RunSummary } from '@/api/types'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import CopyText from '@/components/CopyText.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import OnboardingChecklist from '@/components/onboarding/OnboardingChecklist.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import { fmtDuration, fmtLatency, fmtSpeed, fmtTime, fromNow, runTriggerLabel } from '@/utils/format'
import { useVisibleInterval } from '@/utils/useMedia'

const TrendChart = defineAsyncComponent(() => import('@/components/TrendChart.vue'))

const router = useRouter()
const { t } = useI18n()
const data = ref<Dashboard | null>(null)
const loading = ref(false)
const lastUpdated = ref<string>('')
const metric = ref<'latency' | 'speed'>('latency')

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    data.value = await dashboardApi.get(silent)
    lastUpdated.value = new Date().toISOString()
  } catch {
    /* 已提示 */
  } finally {
    loading.value = false
  }
}

onMounted(() => load())
useVisibleInterval(() => load(true), 10000)

const stats = computed(() => data.value?.stats)
const cfst = computed(() => data.value?.cfst)
const successRate = computed(() => {
  const s = stats.value
  if (!s || !s.runs24h) return null
  return Math.round((s.success24h / s.runs24h) * 100)
})

function openRun(r: RunSummary) {
  router.push(`/runs/${r.id}`)
}

function onMetric(v: unknown) {
  metric.value = v === 'speed' ? 'speed' : 'latency'
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader :title="t('nav.dashboard')" :description="t('dashboard.description')">
      <span v-if="lastUpdated" class="text-muted-foreground hidden text-xs sm:inline">{{ t('dashboard.autoRefresh', { time: fmtTime(lastUpdated, 'HH:mm:ss') }) }}</span>
      <Button variant="outline" size="sm" :disabled="loading" @click="load()">
        <RefreshCw :class="loading ? 'animate-spin' : ''" />{{ t('common.refresh') }}
      </Button>
    </PageHeader>

    <OnboardingChecklist />

    <Alert v-if="cfst && !cfst.installed" variant="destructive" class="border-destructive/40 bg-destructive/5">
      <TriangleAlert />
      <AlertTitle>{{ t('dashboard.cfstMissing.title') }}</AlertTitle>
      <AlertDescription>
        <p>{{ t('dashboard.cfstMissing.text') }}</p>
        <Button size="sm" variant="destructive" class="mt-2" @click="router.push('/cfst')">{{ t('dashboard.cfstMissing.action') }}<ArrowRight /></Button>
      </AlertDescription>
    </Alert>

    <!-- 统计卡片 -->
    <div v-if="!data" class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-5">
      <Skeleton v-for="i in 5" :key="i" class="h-[118px] rounded-xl" />
    </div>
    <div v-else-if="stats" class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-5">
      <Card class="hover:border-primary/40 cursor-pointer gap-1 py-4 transition-colors" @click="router.push('/tasks')">
        <CardHeader>
          <CardDescription class="flex items-center gap-2"><ListChecks class="size-4" />{{ t('dashboard.stats.tasks') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="stat-number">{{ stats.taskCount }}</div>
          <p class="text-muted-foreground mt-1 text-xs">{{ t('dashboard.stats.enabledTasks', { n: stats.enabledTaskCount }) }}</p>
        </CardContent>
      </Card>
      <Card class="hover:border-primary/40 cursor-pointer gap-1 py-4 transition-colors" @click="router.push('/runs')">
        <CardHeader>
          <CardDescription class="flex items-center gap-2"><Activity class="size-4" />{{ t('dashboard.stats.runs24h') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="flex items-baseline gap-2 stat-number">
            <span class="text-success">{{ stats.success24h }}</span>
            <span class="text-muted-foreground text-base font-normal">/</span>
            <span :class="stats.failed24h ? 'text-destructive' : ''">{{ stats.failed24h }}</span>
          </div>
          <p class="text-muted-foreground mt-1 text-xs">
            {{ t('dashboard.stats.totalRuns', { n: stats.runs24h }) }}<template v-if="successRate !== null">{{ t('dashboard.stats.successRate', { n: successRate }) }}</template>
          </p>
        </CardContent>
      </Card>
      <Card class="hover:border-primary/40 cursor-pointer gap-1 py-4 transition-colors" @click="router.push('/accounts')">
        <CardHeader>
          <CardDescription class="flex items-center gap-2"><UserRoundKey class="size-4" />{{ t('dashboard.stats.accounts') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="stat-number">{{ stats.accountCount }}</div>
          <p class="text-muted-foreground mt-1 text-xs">{{ t('dashboard.stats.managedRecords', { n: data.records?.length ?? 0 }) }}</p>
        </CardContent>
      </Card>
      <Card class="hover:border-primary/40 cursor-pointer gap-1 py-4 transition-colors" @click="router.push('/notifiers')">
        <CardHeader>
          <CardDescription class="flex items-center gap-2"><Bell class="size-4" />{{ t('dashboard.stats.notifiers') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="stat-number">{{ stats.notifierCount }}</div>
          <p class="text-muted-foreground mt-1 text-xs">{{ t('dashboard.stats.notifierHint') }}</p>
        </CardContent>
      </Card>
      <Card
        :class="[
          'cursor-pointer gap-1 py-4 transition-colors sm:col-span-2 xl:col-span-1',
          cfst?.installed ? 'hover:border-primary/40' : 'border-destructive/50 bg-destructive/5',
        ]"
        @click="router.push('/cfst')"
      >
        <CardHeader>
          <CardDescription class="flex items-center gap-2"><CloudDownload class="size-4" />{{ t('dashboard.stats.cfstVersion') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <div :class="['stat-number', cfst?.installed ? '' : 'text-destructive tracking-normal']">
            {{ cfst?.installed ? cfst.version || t('dashboard.stats.installed') : t('dashboard.stats.notInstalled') }}
          </div>
          <p class="text-muted-foreground mt-1 text-xs">CloudflareSpeedTest</p>
        </CardContent>
      </Card>
    </div>

    <template v-if="data">
      <!-- 运行中 -->
      <Card v-if="data.active?.length" class="border-info/30 gap-3 py-4">
        <CardHeader>
          <CardTitle class="flex items-center gap-2">
            <span class="relative flex size-2">
              <span class="bg-info absolute inline-flex size-full animate-ping rounded-full opacity-75" />
              <span class="bg-info relative inline-flex size-2 rounded-full" />
            </span>
            {{ t('dashboard.active.title') }}
          </CardTitle>
        </CardHeader>
        <CardContent class="grid grid-cols-1 gap-2 px-5">
          <button
            v-for="r in data.active"
            :key="r.id"
            type="button"
            class="hover:bg-accent/60 group grid grid-cols-1 gap-2 rounded-lg border p-3 text-left transition-colors"
            @click="openRun(r)"
          >
            <div class="flex flex-wrap items-center gap-2">
              <StatusBadge :status="r.status" />
              <span class="font-medium">{{ r.taskName }}</span>
              <span class="text-muted-foreground text-xs">
                {{ runTriggerLabel[r.trigger] ?? r.trigger }} · {{ r.startedAt ? t('dashboard.active.startedAt', { time: fromNow(r.startedAt) }) : t('dashboard.active.queuedAt', { time: fromNow(r.createdAt) }) }}
              </span>
              <span class="text-primary ml-auto flex items-center gap-1 text-xs font-medium">
                {{ t('dashboard.active.liveLog') }}<ArrowRight class="size-3.5 transition-transform group-hover:translate-x-0.5" />
              </span>
            </div>
            <div class="bg-muted relative h-1 overflow-hidden rounded-full">
              <div v-if="r.status === 'running'" class="bg-info absolute inset-y-0 w-1/3 animate-[indeterminate_1.4s_ease-in-out_infinite] rounded-full" />
            </div>
          </button>
        </CardContent>
      </Card>

      <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <!-- 当前记录 -->
        <Card class="gap-0 py-0 xl:col-span-2">
          <CardHeader class="border-b py-3 [.border-b]:pb-3">
            <CardTitle class="flex items-center gap-2"><Globe class="text-muted-foreground size-4" />{{ t('dashboard.records.title') }}</CardTitle>
            <CardDescription>{{ t('dashboard.records.description') }}</CardDescription>
          </CardHeader>
          <CardContent class="p-0">
            <EmptyState v-if="!data.records?.length" :icon="Globe" compact :title="t('dashboard.records.emptyTitle')" :description="t('dashboard.records.emptyDesc')">
              <Button v-if="!stats?.taskCount" size="sm" variant="outline" @click="router.push('/welcome')">{{ t('dashboard.quickStart') }}</Button>
              <Button v-else size="sm" variant="outline" @click="router.push('/tasks')">{{ t('dashboard.goRun') }}</Button>
            </EmptyState>
            <div v-else class="max-h-96 overflow-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead class="pl-5">{{ t('dashboard.records.domain') }}</TableHead>
                    <TableHead class="w-16">{{ t('dashboard.records.type') }}</TableHead>
                    <TableHead>{{ t('dashboard.records.value') }}</TableHead>
                    <TableHead>{{ t('dashboard.records.updated') }}</TableHead>
                    <TableHead class="pr-5">{{ t('dashboard.records.task') }}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow v-for="(r, i) in data.records" :key="i">
                    <TableCell class="min-w-48 pl-5 font-medium whitespace-normal break-all">{{ r.fqdn }}</TableCell>
                    <TableCell><ToneBadge>{{ r.type }}</ToneBadge></TableCell>
                    <TableCell class="min-w-36 whitespace-normal"><CopyText :text="r.value" /></TableCell>
                    <TableCell class="text-muted-foreground text-xs" :title="fmtTime(r.updatedAt)">{{ fromNow(r.updatedAt) }}</TableCell>
                    <TableCell class="text-muted-foreground max-w-32 truncate pr-5 text-xs" :title="r.taskName">{{ r.taskName }}</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>

        <!-- 即将执行 -->
        <Card class="gap-0 py-0">
          <CardHeader class="border-b py-3 [.border-b]:pb-3">
            <CardTitle class="flex items-center gap-2"><CalendarClock class="text-muted-foreground size-4" />{{ t('dashboard.upcoming.title') }}</CardTitle>
            <CardDescription>{{ t('dashboard.upcoming.description') }}</CardDescription>
          </CardHeader>
          <CardContent class="p-0">
            <EmptyState v-if="!data.upcoming?.length" :icon="CalendarClock" compact :title="t('dashboard.upcoming.emptyTitle')" :description="t('dashboard.upcoming.emptyDesc')">
              <Button v-if="!stats?.taskCount" size="sm" variant="outline" @click="router.push('/tasks/new')">{{ t('dashboard.newTask') }}</Button>
              <Button v-else size="sm" variant="outline" @click="router.push('/tasks')">{{ t('dashboard.upcoming.manageTasks') }}</Button>
            </EmptyState>
            <ul v-else class="divide-y">
              <li v-for="u in data.upcoming" :key="u.taskId + u.nextRunAt">
                <router-link :to="`/tasks/${u.taskId}`" class="hover:bg-accent/50 flex items-center gap-3 px-5 py-3 transition-colors">
                  <span class="min-w-0 flex-1 truncate text-sm font-medium" :title="u.taskName">{{ u.taskName }}</span>
                  <span class="text-muted-foreground text-xs tabular-nums" :title="fmtTime(u.nextRunAt)">{{ fromNow(u.nextRunAt) }}</span>
                </router-link>
              </li>
            </ul>
          </CardContent>
        </Card>
      </div>

      <!-- 趋势 -->
      <Card class="gap-4">
        <CardHeader>
          <CardTitle class="flex items-center gap-2"><TrendingUp class="text-muted-foreground size-4" />{{ t('dashboard.trend.title') }}</CardTitle>
          <CardDescription>{{ t('dashboard.trend.description') }}</CardDescription>
          <CardAction>
            <Tabs :model-value="metric" @update:model-value="onMetric">
              <TabsList class="h-8">
                <TabsTrigger value="latency" class="text-xs">{{ t('dashboard.trend.latency') }}</TabsTrigger>
                <TabsTrigger value="speed" class="text-xs">{{ t('dashboard.trend.speed') }}</TabsTrigger>
              </TabsList>
            </Tabs>
          </CardAction>
        </CardHeader>
        <CardContent>
          <EmptyState v-if="!data.trend?.length" :icon="TrendingUp" compact :title="t('dashboard.trend.emptyTitle')" :description="t('dashboard.trend.emptyDesc')">
            <Button size="sm" variant="outline" @click="router.push(stats?.taskCount ? '/tasks' : '/welcome')">{{ stats?.taskCount ? t('dashboard.goRun') : t('dashboard.quickStart') }}</Button>
          </EmptyState>
          <TrendChart v-else :data="data.trend" :metric="metric" />
        </CardContent>
      </Card>

      <!-- 最近执行 -->
      <Card class="gap-0 py-0">
        <CardHeader class="border-b py-3 [.border-b]:pb-3">
          <CardTitle class="flex items-center gap-2"><History class="text-muted-foreground size-4" />{{ t('dashboard.recent.title') }}</CardTitle>
          <CardAction>
            <Button variant="ghost" size="sm" @click="router.push('/runs')">{{ t('common.viewAll') }}<ArrowRight /></Button>
          </CardAction>
        </CardHeader>
        <CardContent class="p-0">
          <EmptyState v-if="!data.lastRuns?.length" :icon="History" compact :title="t('dashboard.recent.emptyTitle')">
            <Button v-if="!stats?.taskCount" size="sm" variant="outline" @click="router.push('/welcome')">{{ t('dashboard.quickStart') }}</Button>
            <Button v-else size="sm" variant="outline" @click="router.push('/tasks')">{{ t('dashboard.goRun') }}</Button>
          </EmptyState>
          <Table v-else>
            <TableHeader>
              <TableRow>
                <TableHead class="pl-5">{{ t('dashboard.recent.status') }}</TableHead>
                <TableHead>{{ t('dashboard.recent.task') }}</TableHead>
                <TableHead>{{ t('dashboard.recent.bestIp') }}</TableHead>
                <TableHead class="text-right">{{ t('dashboard.recent.latencySpeed') }}</TableHead>
                <TableHead class="text-right">{{ t('dashboard.recent.duration') }}</TableHead>
                <TableHead class="pr-5 text-right">{{ t('dashboard.recent.time') }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="r in data.lastRuns" :key="r.id" class="cursor-pointer" @click="openRun(r)">
                <TableCell class="pl-5"><StatusBadge :status="r.status" /></TableCell>
                <TableCell class="min-w-40 whitespace-normal wrap-anywhere">
                  <div class="line-clamp-2 font-medium" :title="r.taskName">{{ r.taskName }}</div>
                  <div class="text-muted-foreground text-xs">{{ runTriggerLabel[r.trigger] ?? r.trigger }}</div>
                </TableCell>
                <TableCell class="text-code min-w-36 font-mono whitespace-normal break-all">
                  <div v-if="r.bestIPv4">{{ r.bestIPv4 }}</div>
                  <div v-if="r.bestIPv6">{{ r.bestIPv6 }}</div>
                  <span v-if="!r.bestIPv4 && !r.bestIPv6" class="text-muted-foreground">-</span>
                </TableCell>
                <TableCell class="text-right text-xs tabular-nums">
                  <div>{{ fmtLatency(r.bestLatency) }}</div>
                  <div class="text-muted-foreground">{{ fmtSpeed(r.bestSpeed) }}</div>
                </TableCell>
                <TableCell class="text-muted-foreground text-right text-xs tabular-nums">{{ fmtDuration(r.durationMs) }}</TableCell>
                <TableCell class="text-muted-foreground pr-5 text-right text-xs" :title="fmtTime(r.startedAt || r.createdAt)">
                  {{ fromNow(r.startedAt || r.createdAt) }}
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
