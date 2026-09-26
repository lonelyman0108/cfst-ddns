<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
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
import { fmtDuration, fmtLatency, fmtSpeed, fmtTime, fromNow, isRunActive, runMessage, runStatusMeta, runTriggerLabel } from '@/utils/format'

const { t } = useI18n()
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
const statusOptions = computed(() => Object.entries(runStatusMeta).map(([value, m]) => ({ value, label: m.label })))

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
  const ok = await confirm({ title: t('runs.list.deleteTitle', { id: r.id }), description: t('runs.list.deleteDesc'), confirmText: t('common.delete'), destructive: true })
  if (!ok) return
  try {
    await runsApi.remove(r.id)
    toast.success(t('runs.list.deleted'))
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
    title: t('runs.list.purgeConfirmTitle'),
    description: t('runs.list.purgeConfirmDesc', { days }),
    confirmText: t('runs.list.purgeBtn'),
    destructive: true,
  })
  if (!ok) return
  purge.loading = true
  try {
    const r = await runsApi.purge(days)
    toast.success(t('runs.list.purged', { n: r.deleted ?? 0 }))
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
    <PageHeader :title="t('runs.list.title')" :description="t('runs.list.desc')">
      <Button variant="outline" size="sm" :disabled="loading" @click="load"><RefreshCw :class="loading ? 'animate-spin' : ''" />{{ t('common.refresh') }}</Button>
      <Button variant="outline" size="sm" class="text-destructive hover:text-destructive" @click="purge.open = true"><Eraser />{{ t('runs.list.purge') }}</Button>
    </PageHeader>

    <div class="flex flex-wrap gap-2">
      <Select :model-value="query.taskId" @update:model-value="setTask">
        <SelectTrigger class="w-48"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem :value="ALL">{{ t('runs.list.allTasks') }}</SelectItem>
          <SelectSeparator v-if="tasks.length" />
          <SelectItem v-for="t in tasks" :key="t.id" :value="String(t.id)">{{ t.name }}</SelectItem>
        </SelectContent>
      </Select>
      <Select :model-value="query.status" @update:model-value="setStatus">
        <SelectTrigger class="w-36"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem :value="ALL">{{ t('runs.list.allStatus') }}</SelectItem>
          <SelectSeparator />
          <SelectItem v-for="s in statusOptions" :key="s.value" :value="s.value">{{ s.label }}</SelectItem>
        </SelectContent>
      </Select>
    </div>

    <Card class="gap-0 overflow-hidden py-0">
      <div v-if="!loaded" class="grid grid-cols-1 gap-3 p-5"><Skeleton v-for="i in 6" :key="i" class="h-10" /></div>
      <EmptyState
        v-else-if="!items.length && (query.taskId !== ALL || query.status !== ALL)"
        :icon="History"
        :title="t('runs.list.noMatch')"
        :description="t('runs.list.noMatchDesc')"
      >
        <Button size="sm" variant="outline" @click="(query.taskId = ALL), (query.status = ALL), applyFilter()">{{ t('runs.list.clearFilter') }}</Button>
      </EmptyState>
      <EmptyState v-else-if="!items.length" :icon="History" :title="t('runs.list.emptyTitle')" :description="t('runs.list.emptyDesc')">
        <Button size="sm" variant="outline" @click="router.push('/tasks')">{{ t('runs.list.goTasks') }}</Button>
      </EmptyState>
      <Table v-else>
        <TableHeader>
          <TableRow class="bg-muted/30 hover:bg-muted/30">
            <TableHead class="w-16 pl-5">#</TableHead>
            <TableHead>{{ t('common.status') }}</TableHead>
            <TableHead>{{ t('runs.list.colTask') }}</TableHead>
            <TableHead>{{ t('runs.list.colBestIp') }}</TableHead>
            <TableHead class="text-right">{{ t('runs.list.colLatencySpeed') }}</TableHead>
            <TableHead class="hidden xl:table-cell text-center">{{ t('runs.list.colChanged') }}</TableHead>
            <TableHead>{{ t('runs.list.colStartedAt') }}</TableHead>
            <TableHead class="hidden xl:table-cell text-right">{{ t('runs.duration') }}</TableHead>
            <TableHead class="hidden xl:table-cell">{{ t('runs.message') }}</TableHead>
            <TableHead class="w-28 pr-5 text-right">{{ t('common.actions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="r in items" :key="r.id">
            <TableCell class="text-muted-foreground pl-5 font-mono text-code">{{ r.id }}</TableCell>
            <TableCell>
              <div class="flex flex-wrap items-center gap-1.5">
                <StatusBadge :status="r.status" />
                <ToneBadge v-if="r.dryRun" tone="info" :title="t('runs.dryRunTip')">{{ t('runs.dryRun') }}</ToneBadge>
              </div>
            </TableCell>
            <TableCell class="min-w-40 whitespace-normal wrap-anywhere">
              <router-link :to="`/tasks/${r.taskId}`" class="line-clamp-2 font-medium hover:underline" :title="r.taskName">{{ r.taskName }}</router-link>
              <div class="text-muted-foreground text-xs">{{ runTriggerLabel[r.trigger] ?? r.trigger }}</div>
              <!-- 窄屏隐藏「说明」列时显示在任务名下方 -->
              <div v-if="r.message" class="text-muted-foreground line-clamp-2 text-xs xl:hidden" :title="runMessage(r)">{{ runMessage(r) }}</div>
            </TableCell>
            <TableCell class="text-code min-w-32 font-mono whitespace-normal break-all">
              <div v-if="r.bestIPv4">{{ r.bestIPv4 }}</div>
              <div v-if="r.bestIPv6">{{ r.bestIPv6 }}</div>
              <span v-if="!r.bestIPv4 && !r.bestIPv6" class="text-muted-foreground">-</span>
            </TableCell>
            <TableCell class="text-right text-xs tabular-nums">
              <div>{{ fmtLatency(r.bestLatency) }}</div>
              <div class="text-muted-foreground">{{ fmtSpeed(r.bestSpeed) }}</div>
            </TableCell>
            <TableCell class="hidden xl:table-cell text-center">
              <ToneBadge v-if="r.changed" tone="success">{{ t('common.yes') }}</ToneBadge>
              <span v-else class="text-muted-foreground text-xs">{{ t('common.no') }}</span>
            </TableCell>
            <TableCell class="text-xs tabular-nums" :title="fromNow(r.startedAt || r.createdAt)">
              <div>{{ fmtTime(r.startedAt || r.createdAt, 'YYYY-MM-DD') }}</div>
              <div class="text-muted-foreground">{{ fmtTime(r.startedAt || r.createdAt, 'HH:mm:ss') }}</div>
            </TableCell>
            <TableCell class="text-muted-foreground hidden xl:table-cell text-right text-xs tabular-nums">{{ fmtDuration(r.durationMs) }}</TableCell>
            <TableCell class="text-muted-foreground hidden xl:table-cell max-w-56 min-w-40 text-xs whitespace-normal" :title="runMessage(r)">
              <span class="line-clamp-2 break-words">{{ runMessage(r) || '-' }}</span>
            </TableCell>
            <TableCell class="pr-5">
              <div class="flex justify-end gap-0.5">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" class="size-8" @click="view(r)"><Eye /></Button>
                  </TooltipTrigger>
                  <TooltipContent>{{ t('runs.list.viewDetails') }}</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" class="size-8" @click="router.push(`/runs/${r.id}`)"><SquareArrowOutUpRight /></Button>
                  </TooltipTrigger>
                  <TooltipContent>{{ t('common.openInPage') }}</TooltipContent>
                </Tooltip>
                <Tooltip v-if="!isRunActive(r.status)">
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" class="text-muted-foreground hover:text-destructive size-8" @click="remove(r)"><Trash2 /></Button>
                  </TooltipTrigger>
                  <TooltipContent>{{ t('common.delete') }}</TooltipContent>
                </Tooltip>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </Card>

    <div v-if="total > 0" class="flex flex-wrap items-center justify-between gap-3 text-sm">
      <div class="text-muted-foreground flex items-center gap-2">
        {{ t('runs.list.pageInfo', { total }) }}
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
          <DialogTitle>{{ t('runs.list.purgeTitle') }}</DialogTitle>
          <DialogDescription>{{ t('runs.list.purgeDesc') }}</DialogDescription>
        </DialogHeader>
        <FormItem :label="t('runs.list.purgeDays')">
          <NumInput v-model="purge.days" :min="1" :max="3650" class="max-w-40" />
        </FormItem>
        <DialogFooter>
          <Button variant="outline" @click="purge.open = false">{{ t('common.cancel') }}</Button>
          <Button variant="destructive" :disabled="purge.loading" @click="doPurge"><Loader2 v-if="purge.loading" class="animate-spin" />{{ t('runs.list.purgeBtn') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <RunSheet v-model="sheetOpen" :run-id="sheetRunId" @done="load" />
  </div>
</template>
