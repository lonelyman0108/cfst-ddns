<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Copy, EllipsisVertical, FlaskConical, ListChecks, Pencil, Play, Plus, RefreshCw, Trash2 } from '@lucide/vue'
import { errorStatus, runsApi, tasksApi } from '@/api'
import type { Task } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import RunSheet from '@/components/RunSheet.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import { confirm } from '@/composables/useConfirm'
import { describeCron } from '@/utils/cron'
import { fmtTime, fromNow, ipTypeLabel } from '@/utils/format'
import { useVisibleInterval } from '@/utils/useMedia'

const { t } = useI18n()
const router = useRouter()
const tasks = ref<Task[]>([])
const loaded = ref(false)
const loading = ref(false)
const toggling = ref<Record<number, boolean>>({})
const running = ref<Record<number, boolean>>({})

const sheetOpen = ref(false)
const sheetRunId = ref<number | null>(null)

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    tasks.value = (await tasksApi.list()) ?? []
    loaded.value = true
  } catch {
    /* 已提示 */
  } finally {
    loading.value = false
  }
}

onMounted(() => load())
useVisibleInterval(() => {
  if (!sheetOpen.value) load(true)
}, 15000)

async function toggle(task: Task, v: boolean) {
  toggling.value[task.id] = true
  try {
    Object.assign(task, await tasksApi.setEnabled(task.id, v))
    toast.success(t(v ? 'tasks.list.enabledToast' : 'tasks.list.disabledToast', { name: task.name }))
  } catch {
    /* 已提示 */
  } finally {
    toggling.value[task.id] = false
  }
}

function openRun(runId: number) {
  sheetRunId.value = runId
  sheetOpen.value = true
}

async function runNow(task: Task, dryRun = false) {
  running.value[task.id] = true
  try {
    const { runId } = await tasksApi.run(task.id, { dryRun })
    toast.success(t('tasks.list.queued', { name: task.name }), dryRun ? { description: t('tasks.list.dryRunToast') } : undefined)
    openRun(runId)
    load(true)
  } catch (e) {
    // 409：已在运行或排队，打开正在进行的执行
    if (errorStatus(e) === 409) {
      const active = await runsApi.active().catch(() => [])
      const r = active.find((a) => a.taskId === task.id)
      if (r) openRun(r.id)
    }
  } finally {
    running.value[task.id] = false
  }
}

async function clone(task: Task) {
  try {
    const c = await tasksApi.clone(task.id)
    toast.success(t('tasks.list.cloned', { name: c.name }))
    router.push(`/tasks/${c.id}`)
  } catch {
    /* 已提示 */
  }
}

async function remove(task: Task) {
  const ok = await confirm({
    title: t('tasks.list.deleteTitle', { name: task.name }),
    description: t('tasks.list.deleteDesc'),
    confirmText: t('common.delete'),
    destructive: true,
  })
  if (!ok) return
  try {
    await tasksApi.remove(task.id)
    toast.success(t('tasks.list.deleted'))
    load(true)
  } catch {
    /* 已提示 */
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader :title="$t('tasks.list.title')" :description="$t('tasks.list.desc')">
      <Button variant="outline" size="sm" :disabled="loading" @click="load()">
        <RefreshCw :class="loading ? 'animate-spin' : ''" />{{ $t('common.refresh') }}
      </Button>
      <Button size="sm" @click="router.push('/tasks/new')"><Plus />{{ $t('tasks.newTask') }}</Button>
    </PageHeader>

    <Card class="gap-0 overflow-hidden py-0">
      <div v-if="!loaded" class="grid grid-cols-1 gap-3 p-5">
        <Skeleton v-for="i in 4" :key="i" class="h-10" />
      </div>
      <EmptyState v-else-if="!tasks.length" :icon="ListChecks" :title="$t('tasks.list.emptyTitle')" :description="$t('tasks.list.emptyDesc')">
        <Button size="sm" @click="router.push('/tasks/new')"><Plus />{{ $t('tasks.newTask') }}</Button>
        <Button size="sm" variant="outline" @click="router.push({ path: '/welcome', query: { step: 'task' } })">{{ $t('tasks.list.fromTemplate') }}</Button>
      </EmptyState>
      <Table v-else>
        <TableHeader>
          <TableRow class="bg-muted/30 hover:bg-muted/30">
            <TableHead class="pl-5">{{ $t('common.name') }}</TableHead>
            <TableHead class="w-16">{{ $t('common.enable') }}</TableHead>
            <TableHead>{{ $t('tasks.schedule') }}</TableHead>
            <TableHead>{{ $t('tasks.ipType') }}</TableHead>
            <TableHead class="hidden xl:table-cell text-center">{{ $t('tasks.list.colTargets') }}</TableHead>
            <TableHead>{{ $t('tasks.list.colNextRun') }}</TableHead>
            <TableHead>{{ $t('tasks.list.colLastRun') }}</TableHead>
            <TableHead class="w-28 pr-5 text-right">{{ $t('common.actions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="t in tasks" :key="t.id">
            <TableCell class="min-w-40 whitespace-normal wrap-anywhere pl-5">
              <div class="flex items-center gap-2">
                <router-link :to="`/tasks/${t.id}`" class="line-clamp-2 font-medium hover:underline" :title="t.name">{{ t.name }}</router-link>
                <StatusBadge v-if="t.running" status="running" />
              </div>
            </TableCell>
            <TableCell>
              <Switch :model-value="t.enabled" :disabled="toggling[t.id]" @update:model-value="toggle(t, $event)" />
            </TableCell>
            <TableCell>
              <span v-if="!t.cron" class="text-muted-foreground">{{ $t('cron.presets.manual') }}</span>
              <Tooltip v-else>
                <TooltipTrigger as-child><span class="cursor-default">{{ describeCron(t.cron) }}</span></TooltipTrigger>
                <TooltipContent class="font-mono">{{ t.cron }}</TooltipContent>
              </Tooltip>
            </TableCell>
            <TableCell><ToneBadge>{{ ipTypeLabel[t.ipType] ?? t.ipType }}</ToneBadge></TableCell>
            <TableCell class="hidden xl:table-cell text-center tabular-nums">{{ t.targets?.length ?? 0 }}</TableCell>
            <TableCell class="text-muted-foreground text-xs">
              <span v-if="t.nextRunAt && t.enabled" :title="fmtTime(t.nextRunAt)">{{ fromNow(t.nextRunAt) }}</span>
              <span v-else>-</span>
            </TableCell>
            <TableCell>
              <button v-if="t.lastRun" type="button" class="flex items-center gap-2" @click="openRun(t.lastRun.id)">
                <StatusBadge :status="t.lastRun.status" />
                <span class="text-muted-foreground text-xs">{{ fromNow(t.lastRun.finishedAt || t.lastRun.startedAt || t.lastRun.createdAt) }}</span>
              </button>
              <span v-else class="text-muted-foreground text-xs">{{ $t('tasks.list.neverRun') }}</span>
            </TableCell>
            <TableCell class="pr-5">
              <div class="flex items-center justify-end gap-1">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" class="text-primary size-8" :disabled="running[t.id]" @click="runNow(t)">
                      <Play />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{{ $t('tasks.list.runNow') }}</TooltipContent>
                </Tooltip>
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <Button variant="ghost" size="icon" class="size-8"><EllipsisVertical /></Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" class="w-44">
                    <DropdownMenuItem :disabled="running[t.id]" @select="runNow(t, true)"><FlaskConical />{{ $t('runs.dryRun') }}</DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem @select="router.push(`/tasks/${t.id}`)"><Pencil />{{ $t('common.edit') }}</DropdownMenuItem>
                    <DropdownMenuItem @select="clone(t)"><Copy />{{ $t('tasks.list.clone') }}</DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem variant="destructive" @select="remove(t)"><Trash2 />{{ $t('common.delete') }}</DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </Card>

    <RunSheet v-model="sheetOpen" :run-id="sheetRunId" @done="load(true)" />
  </div>
</template>
