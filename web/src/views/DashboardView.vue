<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
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
import StatusBadge from '@/components/StatusBadge.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import { fmtDuration, fmtLatency, fmtSpeed, fmtTime, fromNow, runTriggerLabel } from '@/utils/format'
import { useVisibleInterval } from '@/utils/useMedia'

const TrendChart = defineAsyncComponent(() => import('@/components/TrendChart.vue'))

const router = useRouter()
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
    <PageHeader title="仪表盘" description="测速任务、DNS 记录与执行情况一览">
      <span v-if="lastUpdated" class="text-muted-foreground hidden text-xs sm:inline">每 10 秒自动刷新 · {{ fmtTime(lastUpdated, 'HH:mm:ss') }}</span>
      <Button variant="outline" size="sm" :disabled="loading" @click="load()">
        <RefreshCw :class="loading ? 'animate-spin' : ''" />刷新
      </Button>
    </PageHeader>

    <Alert v-if="cfst && !cfst.installed" variant="destructive" class="border-destructive/40 bg-destructive/5">
      <TriangleAlert />
      <AlertTitle>尚未安装 CloudflareSpeedTest（cfst）</AlertTitle>
      <AlertDescription>
        <p>任务将无法执行测速，请先安装 cfst。</p>
        <Button size="sm" variant="destructive" class="mt-2" @click="router.push('/cfst')">前往安装<ArrowRight /></Button>
      </AlertDescription>
    </Alert>

    <!-- 统计卡片 -->
    <div v-if="!data" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
      <Skeleton v-for="i in 5" :key="i" class="h-[118px] rounded-xl" />
    </div>
    <div v-else-if="stats" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
      <Card class="hover:border-primary/40 cursor-pointer gap-1 py-4 transition-colors" @click="router.push('/tasks')">
        <CardHeader>
          <CardDescription class="flex items-center gap-2"><ListChecks class="size-4" />任务</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="stat-number">{{ stats.taskCount }}</div>
          <p class="text-muted-foreground mt-1 text-xs">{{ stats.enabledTaskCount }} 个已启用</p>
        </CardContent>
      </Card>
      <Card class="hover:border-primary/40 cursor-pointer gap-1 py-4 transition-colors" @click="router.push('/runs')">
        <CardHeader>
          <CardDescription class="flex items-center gap-2"><Activity class="size-4" />24 小时执行</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="flex items-baseline gap-2 stat-number">
            <span class="text-success">{{ stats.success24h }}</span>
            <span class="text-muted-foreground text-base font-normal">/</span>
            <span :class="stats.failed24h ? 'text-destructive' : ''">{{ stats.failed24h }}</span>
          </div>
          <p class="text-muted-foreground mt-1 text-xs">
            共 {{ stats.runs24h }} 次<template v-if="successRate !== null"> · 成功率 {{ successRate }}%</template>
          </p>
        </CardContent>
      </Card>
      <Card class="hover:border-primary/40 cursor-pointer gap-1 py-4 transition-colors" @click="router.push('/accounts')">
        <CardHeader>
          <CardDescription class="flex items-center gap-2"><UserRoundKey class="size-4" />DNS 账号</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="stat-number">{{ stats.accountCount }}</div>
          <p class="text-muted-foreground mt-1 text-xs">{{ data.records?.length ?? 0 }} 条受管记录</p>
        </CardContent>
      </Card>
      <Card class="hover:border-primary/40 cursor-pointer gap-1 py-4 transition-colors" @click="router.push('/notifiers')">
        <CardHeader>
          <CardDescription class="flex items-center gap-2"><Bell class="size-4" />通知渠道</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="stat-number">{{ stats.notifierCount }}</div>
          <p class="text-muted-foreground mt-1 text-xs">成功 / 失败 / IP 变化时推送</p>
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
          <CardDescription class="flex items-center gap-2"><CloudDownload class="size-4" />cfst 版本</CardDescription>
        </CardHeader>
        <CardContent>
          <div :class="['stat-number', cfst?.installed ? '' : 'text-destructive tracking-normal']">
            {{ cfst?.installed ? cfst.version || '已安装' : '未安装' }}
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
            运行中
          </CardTitle>
        </CardHeader>
        <CardContent class="grid gap-2 px-5">
          <button
            v-for="r in data.active"
            :key="r.id"
            type="button"
            class="hover:bg-accent/60 group grid gap-2 rounded-lg border p-3 text-left transition-colors"
            @click="openRun(r)"
          >
            <div class="flex flex-wrap items-center gap-2">
              <StatusBadge :status="r.status" />
              <span class="font-medium">{{ r.taskName }}</span>
              <span class="text-muted-foreground text-xs">
                {{ runTriggerLabel[r.trigger] ?? r.trigger }} · {{ r.startedAt ? `开始于 ${fromNow(r.startedAt)}` : `排队于 ${fromNow(r.createdAt)}` }}
              </span>
              <span class="text-primary ml-auto flex items-center gap-1 text-xs font-medium">
                实时日志<ArrowRight class="size-3.5 transition-transform group-hover:translate-x-0.5" />
              </span>
            </div>
            <div class="bg-muted relative h-1 overflow-hidden rounded-full">
              <div v-if="r.status === 'running'" class="bg-info absolute inset-y-0 w-1/3 animate-[indeterminate_1.4s_ease-in-out_infinite] rounded-full" />
            </div>
          </button>
        </CardContent>
      </Card>

      <div class="grid gap-4 xl:grid-cols-3">
        <!-- 当前记录 -->
        <Card class="gap-0 py-0 xl:col-span-2">
          <CardHeader class="border-b py-3 [.border-b]:pb-3">
            <CardTitle class="flex items-center gap-2"><Globe class="text-muted-foreground size-4" />当前记录</CardTitle>
            <CardDescription>每个目标记录最近一次写入的值</CardDescription>
          </CardHeader>
          <CardContent class="p-0">
            <EmptyState v-if="!data.records?.length" :icon="Globe" compact title="暂无记录" description="任务执行成功后，这里会显示各域名当前解析到的 IP" />
            <div v-else class="max-h-96 overflow-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead class="pl-5">域名</TableHead>
                    <TableHead class="w-16">类型</TableHead>
                    <TableHead>值</TableHead>
                    <TableHead>更新</TableHead>
                    <TableHead class="pr-5">任务</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow v-for="(r, i) in data.records" :key="i">
                    <TableCell class="pl-5 font-medium">{{ r.fqdn }}</TableCell>
                    <TableCell><ToneBadge>{{ r.type }}</ToneBadge></TableCell>
                    <TableCell class="max-w-56"><CopyText :text="r.value" /></TableCell>
                    <TableCell class="text-muted-foreground text-xs" :title="fmtTime(r.updatedAt)">{{ fromNow(r.updatedAt) }}</TableCell>
                    <TableCell class="text-muted-foreground max-w-32 truncate pr-5 text-xs">{{ r.taskName }}</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>

        <!-- 即将执行 -->
        <Card class="gap-0 py-0">
          <CardHeader class="border-b py-3 [.border-b]:pb-3">
            <CardTitle class="flex items-center gap-2"><CalendarClock class="text-muted-foreground size-4" />即将执行</CardTitle>
            <CardDescription>按 cron 排程的下一次执行</CardDescription>
          </CardHeader>
          <CardContent class="p-0">
            <EmptyState v-if="!data.upcoming?.length" :icon="CalendarClock" compact title="没有排程" description="为任务设置执行周期后显示">
              <Button size="sm" variant="outline" @click="router.push('/tasks')">管理任务</Button>
            </EmptyState>
            <ul v-else class="divide-y">
              <li v-for="u in data.upcoming" :key="u.taskId + u.nextRunAt">
                <router-link :to="`/tasks/${u.taskId}`" class="hover:bg-accent/50 flex items-center gap-3 px-5 py-3 transition-colors">
                  <span class="min-w-0 flex-1 truncate text-sm font-medium">{{ u.taskName }}</span>
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
          <CardTitle class="flex items-center gap-2"><TrendingUp class="text-muted-foreground size-4" />延迟与速度趋势</CardTitle>
          <CardDescription>最近 50 次成功执行的最优值</CardDescription>
          <CardAction>
            <Tabs :model-value="metric" @update:model-value="onMetric">
              <TabsList class="h-8">
                <TabsTrigger value="latency" class="text-xs">延迟</TabsTrigger>
                <TabsTrigger value="speed" class="text-xs">速度</TabsTrigger>
              </TabsList>
            </Tabs>
          </CardAction>
        </CardHeader>
        <CardContent>
          <EmptyState v-if="!data.trend?.length" :icon="TrendingUp" compact title="暂无数据" description="任务成功执行后会在这里绘制趋势" />
          <TrendChart v-else :data="data.trend" :metric="metric" />
        </CardContent>
      </Card>

      <!-- 最近执行 -->
      <Card class="gap-0 py-0">
        <CardHeader class="border-b py-3 [.border-b]:pb-3">
          <CardTitle class="flex items-center gap-2"><History class="text-muted-foreground size-4" />最近执行</CardTitle>
          <CardAction>
            <Button variant="ghost" size="sm" @click="router.push('/runs')">查看全部<ArrowRight /></Button>
          </CardAction>
        </CardHeader>
        <CardContent class="p-0">
          <EmptyState v-if="!data.lastRuns?.length" :icon="History" compact title="暂无执行记录">
            <Button size="sm" variant="outline" @click="router.push('/tasks')">去执行任务</Button>
          </EmptyState>
          <Table v-else>
            <TableHeader>
              <TableRow>
                <TableHead class="pl-5">状态</TableHead>
                <TableHead>任务</TableHead>
                <TableHead>最优 IP</TableHead>
                <TableHead class="text-right">延迟 / 速度</TableHead>
                <TableHead class="text-right">耗时</TableHead>
                <TableHead class="pr-5 text-right">时间</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="r in data.lastRuns" :key="r.id" class="cursor-pointer" @click="openRun(r)">
                <TableCell class="pl-5"><StatusBadge :status="r.status" /></TableCell>
                <TableCell>
                  <div class="font-medium">{{ r.taskName }}</div>
                  <div class="text-muted-foreground text-xs">{{ runTriggerLabel[r.trigger] ?? r.trigger }}</div>
                </TableCell>
                <TableCell class="font-mono text-code">
                  <div v-if="r.bestIPv4">{{ r.bestIPv4 }}</div>
                  <div v-if="r.bestIPv6" class="max-w-52 truncate">{{ r.bestIPv6 }}</div>
                  <span v-if="!r.bestIPv4 && !r.bestIPv6" class="text-muted-foreground">-</span>
                </TableCell>
                <TableCell class="text-right text-xs tabular-nums">{{ fmtLatency(r.bestLatency) }} / {{ fmtSpeed(r.bestSpeed) }}</TableCell>
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
