<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { ChevronLeft, ChevronRight, Eraser, Eye, History, Loader2, RefreshCw, SquareArrowOutUpRight, Trash2 } from '@lucide/vue'
import { runsApi, tasksApi } from '@/api'
import type { RunStatus, RunSummary, Task } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectSeparator, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import EmptyState from '@/components/EmptyState.vue'
import FormItem from '@/components/FormItem.vue'
import NumInput from '@/components/NumInput.vue'
import PageHeader from '@/components/PageHeader.vue'
import RunSheet from '@/components/RunSheet.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import { confirm } from '@/composables/useConfirm'
import { fmtDuration, fmtLatency, fmtSpeed, fmtTime, fromNow, isRunActive, runStatusMeta, runTriggerLabel } from '@/utils/format'

const route = useRoute()
const router = useRouter()

const ALL = '__all__'
const tasks = ref<Task[]>([])
const items = ref<RunSummary[]>([])
const total = ref(0)
const loading = ref(false)
const loaded = ref(false)

const query = reactive({
  taskId: route.query.taskId ? String(route.query.taskId) : ALL,
  status: typeof route.query.status === 'string' ? route.query.status : ALL,
  page: 1,
  size: 20,
})

const pages = computed(() => Math.max(1, Math.ceil(total.value / query.size)))
const statusOptions = Object.entries(runStatusMeta).map(([value, m]) => ({ value, label: m.label }))

const sheetOpen = ref(false)
const sheetRunId = ref<number | null>(null)

async function load() {
  loading.value = true
  try {
    const r = await runsApi.list({
      taskId: query.taskId !== ALL ? Number(query.taskId) : undefined,
      status: query.status !== ALL ? (query.status as RunStatus) : undefined,
      page: query.page,
      size: query.size,
    })
    items.value = r.items ?? []
    total.value = r.total ?? 0
    loaded.value = true
  } catch {
    /* 已提示 */
  } finally {
    loading.value = false
  }
}

function applyFilter() {
  query.page = 1
  router.replace({
    query: {
      ...(query.taskId !== ALL ? { taskId: query.taskId } : {}),
      ...(query.status !== ALL ? { status: query.status } : {}),
    },
  })
  load()
}

function setTask(v: unknown) {
  query.taskId = String(v ?? ALL)
  applyFilter()
}
function setStatus(v: unknown) {
  query.status = String(v ?? ALL)
  applyFilter()
}
function setSize(v: unknown) {
  query.size = Number(v) || 20
  applyFilter()
}
function goPage(p: number) {
  query.page = Math.min(Math.max(1, p), pages.value)
  load()
}

onMounted(() => {
  load()
  tasksApi
    .list()
    .then((t) => (tasks.value = t ?? []))
    .catch(() => {})
})

function view(r: RunSummary) {
  sheetRunId.value = r.id
  sheetOpen.value = true
}

async function remove(r: RunSummary) {
  const ok = await confirm({ title: `删除执行记录 #${r.id}？`, description: '删除后无法恢复。', confirmText: '删除', destructive: true })
  if (!ok) return
  try {
    await runsApi.remove(r.id)
    toast.success('已删除')
    if (items.value.length === 1 && query.page > 1) query.page--
    load()
  } catch {
    /* 已提示 */
  }
}

const purge = reactive({ open: false, days: 30 as number | undefined, loading: false })

async function doPurge() {
  const days = purge.days ?? 30
  const ok = await confirm({
    title: '确认清理',
    description: `将删除 ${days} 天前的全部执行记录，此操作不可恢复。`,
    confirmText: '清理',
    destructive: true,
  })
  if (!ok) return
  purge.loading = true
  try {
    const r = await runsApi.purge(days)
    toast.success(`已清理 ${r.deleted ?? 0} 条记录`)
    purge.open = false
    query.page = 1
    load()
  } catch {
    /* 已提示 */
  } finally {
    purge.loading = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="执行历史" description="每次测速与 DNS 同步的记录">
      <Button variant="outline" size="sm" :disabled="loading" @click="load"><RefreshCw :class="loading ? 'animate-spin' : ''" />刷新</Button>
      <Button variant="outline" size="sm" class="text-destructive hover:text-destructive" @click="purge.open = true"><Eraser />批量清理</Button>
    </PageHeader>

    <div class="flex flex-wrap gap-2">
      <Select :model-value="query.taskId" @update:model-value="setTask">
        <SelectTrigger class="w-48"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem :value="ALL">全部任务</SelectItem>
          <SelectSeparator v-if="tasks.length" />
          <SelectItem v-for="t in tasks" :key="t.id" :value="String(t.id)">{{ t.name }}</SelectItem>
        </SelectContent>
      </Select>
      <Select :model-value="query.status" @update:model-value="setStatus">
        <SelectTrigger class="w-36"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem :value="ALL">全部状态</SelectItem>
          <SelectSeparator />
          <SelectItem v-for="s in statusOptions" :key="s.value" :value="s.value">{{ s.label }}</SelectItem>
        </SelectContent>
      </Select>
    </div>

    <Card class="gap-0 overflow-hidden py-0">
      <div v-if="!loaded" class="grid gap-3 p-5"><Skeleton v-for="i in 6" :key="i" class="h-10" /></div>
      <EmptyState v-else-if="!items.length" :icon="History" title="暂无执行记录" description="执行任务后会在这里记录每次的结果">
        <Button size="sm" variant="outline" @click="router.push('/tasks')">前往任务</Button>
      </EmptyState>
      <Table v-else>
        <TableHeader>
          <TableRow class="bg-muted/30 hover:bg-muted/30">
            <TableHead class="w-16 pl-5">#</TableHead>
            <TableHead>状态</TableHead>
            <TableHead>任务</TableHead>
            <TableHead>最优 IP</TableHead>
            <TableHead class="text-right">延迟 / 速度</TableHead>
            <TableHead class="text-center">变更</TableHead>
            <TableHead>开始时间</TableHead>
            <TableHead class="text-right">耗时</TableHead>
            <TableHead>说明</TableHead>
            <TableHead class="w-28 pr-5 text-right">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="r in items" :key="r.id">
            <TableCell class="text-muted-foreground pl-5 font-mono text-code">{{ r.id }}</TableCell>
            <TableCell><StatusBadge :status="r.status" /></TableCell>
            <TableCell>
              <router-link :to="`/tasks/${r.taskId}`" class="font-medium hover:underline">{{ r.taskName }}</router-link>
              <div class="text-muted-foreground text-xs">{{ runTriggerLabel[r.trigger] ?? r.trigger }}</div>
            </TableCell>
            <TableCell class="font-mono text-code">
              <div v-if="r.bestIPv4">{{ r.bestIPv4 }}</div>
              <div v-if="r.bestIPv6" class="max-w-52 truncate">{{ r.bestIPv6 }}</div>
              <span v-if="!r.bestIPv4 && !r.bestIPv6" class="text-muted-foreground">-</span>
            </TableCell>
            <TableCell class="text-right text-xs tabular-nums">{{ fmtLatency(r.bestLatency) }} / {{ fmtSpeed(r.bestSpeed) }}</TableCell>
            <TableCell class="text-center">
              <ToneBadge v-if="r.changed" tone="success">是</ToneBadge>
              <span v-else class="text-muted-foreground text-xs">否</span>
            </TableCell>
            <TableCell class="text-xs tabular-nums" :title="fromNow(r.startedAt || r.createdAt)">{{ fmtTime(r.startedAt || r.createdAt) }}</TableCell>
            <TableCell class="text-muted-foreground text-right text-xs tabular-nums">{{ fmtDuration(r.durationMs) }}</TableCell>
            <TableCell class="text-muted-foreground max-w-56 truncate text-xs" :title="r.message">{{ r.message || '-' }}</TableCell>
            <TableCell class="pr-5">
              <div class="flex justify-end gap-0.5">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" class="size-8" @click="view(r)"><Eye /></Button>
                  </TooltipTrigger>
                  <TooltipContent>查看详情</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" class="size-8" @click="router.push(`/runs/${r.id}`)"><SquareArrowOutUpRight /></Button>
                  </TooltipTrigger>
                  <TooltipContent>在页面中打开</TooltipContent>
                </Tooltip>
                <Tooltip v-if="!isRunActive(r.status)">
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" class="text-muted-foreground hover:text-destructive size-8" @click="remove(r)"><Trash2 /></Button>
                  </TooltipTrigger>
                  <TooltipContent>删除</TooltipContent>
                </Tooltip>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </Card>

    <div v-if="total > 0" class="flex flex-wrap items-center justify-between gap-3 text-sm">
      <div class="text-muted-foreground flex items-center gap-2">
        共 {{ total }} 条 · 每页
        <Select :model-value="String(query.size)" @update:model-value="setSize">
          <SelectTrigger size="sm" class="w-20"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="s in [20, 50, 100]" :key="s" :value="String(s)">{{ s }}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="flex items-center gap-2">
        <Button variant="outline" size="icon" class="size-8" :disabled="query.page <= 1 || loading" @click="goPage(query.page - 1)"><ChevronLeft /></Button>
        <span class="tabular-nums">{{ query.page }} / {{ pages }}</span>
        <Button variant="outline" size="icon" class="size-8" :disabled="query.page >= pages || loading" @click="goPage(query.page + 1)"><ChevronRight /></Button>
      </div>
    </div>

    <Dialog v-model:open="purge.open">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>批量清理执行记录</DialogTitle>
          <DialogDescription>系统也会按「系统设置」中的保留天数自动清理。</DialogDescription>
        </DialogHeader>
        <FormItem label="删除早于多少天的记录">
          <NumInput v-model="purge.days" :min="1" :max="3650" class="max-w-40" />
        </FormItem>
        <DialogFooter>
          <Button variant="outline" @click="purge.open = false">取消</Button>
          <Button variant="destructive" :disabled="purge.loading" @click="doPurge"><Loader2 v-if="purge.loading" class="animate-spin" />清理</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <RunSheet v-model="sheetOpen" :run-id="sheetRunId" @done="load" />
  </div>
</template>
