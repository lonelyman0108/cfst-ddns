<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
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

async function toggle(t: Task, v: boolean) {
  toggling.value[t.id] = true
  try {
    Object.assign(t, await tasksApi.setEnabled(t.id, v))
    toast.success(v ? `已启用「${t.name}」` : `已停用「${t.name}」`)
  } catch {
    /* 已提示 */
  } finally {
    toggling.value[t.id] = false
  }
}

function openRun(runId: number) {
  sheetRunId.value = runId
  sheetOpen.value = true
}

async function runNow(t: Task, dryRun = false) {
  running.value[t.id] = true
  try {
    const { runId } = await tasksApi.run(t.id, { dryRun })
    toast.success(`「${t.name}」已加入执行队列`, dryRun ? { description: '试运行：只测速，不修改 DNS、不发送通知' } : undefined)
    openRun(runId)
    load(true)
  } catch (e) {
    // 409：已在运行或排队，打开正在进行的执行
    if (errorStatus(e) === 409) {
      const active = await runsApi.active().catch(() => [])
      const r = active.find((a) => a.taskId === t.id)
      if (r) openRun(r.id)
    }
  } finally {
    running.value[t.id] = false
  }
}

async function clone(t: Task) {
  try {
    const c = await tasksApi.clone(t.id)
    toast.success(`已克隆为「${c.name}」`)
    router.push(`/tasks/${c.id}`)
  } catch {
    /* 已提示 */
  }
}

async function remove(t: Task) {
  const ok = await confirm({
    title: `删除任务「${t.name}」？`,
    description: '任务及其执行历史可能被一并删除，此操作不可恢复。已写入的 DNS 记录不会被删除。',
    confirmText: '删除',
    destructive: true,
  })
  if (!ok) return
  try {
    await tasksApi.remove(t.id)
    toast.success('任务已删除')
    load(true)
  } catch {
    /* 已提示 */
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="任务" description="定时测速并把最优 IP 写入 DNS 记录">
      <Button variant="outline" size="sm" :disabled="loading" @click="load()">
        <RefreshCw :class="loading ? 'animate-spin' : ''" />刷新
      </Button>
      <Button size="sm" @click="router.push('/tasks/new')"><Plus />新建任务</Button>
    </PageHeader>

    <Card class="gap-0 overflow-hidden py-0">
      <div v-if="!loaded" class="grid grid-cols-1 gap-3 p-5">
        <Skeleton v-for="i in 4" :key="i" class="h-10" />
      </div>
      <EmptyState v-else-if="!tasks.length" :icon="ListChecks" title="还没有任务" description="创建一个任务：选择测速参数、目标 DNS 记录与执行周期">
        <Button size="sm" @click="router.push('/tasks/new')"><Plus />新建任务</Button>
        <Button size="sm" variant="outline" @click="router.push({ path: '/welcome', query: { step: 'task' } })">从模板创建</Button>
      </EmptyState>
      <Table v-else>
        <TableHeader>
          <TableRow class="bg-muted/30 hover:bg-muted/30">
            <TableHead class="pl-5">名称</TableHead>
            <TableHead class="w-16">启用</TableHead>
            <TableHead>执行周期</TableHead>
            <TableHead>IP 类型</TableHead>
            <TableHead class="hidden xl:table-cell text-center">目标</TableHead>
            <TableHead>下次执行</TableHead>
            <TableHead>上次执行</TableHead>
            <TableHead class="w-28 pr-5 text-right">操作</TableHead>
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
              <span v-if="!t.cron" class="text-muted-foreground">仅手动</span>
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
              <span v-else class="text-muted-foreground text-xs">从未执行</span>
            </TableCell>
            <TableCell class="pr-5">
              <div class="flex items-center justify-end gap-1">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon" class="text-primary size-8" :disabled="running[t.id]" @click="runNow(t)">
                      <Play />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>立即执行</TooltipContent>
                </Tooltip>
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <Button variant="ghost" size="icon" class="size-8"><EllipsisVertical /></Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" class="w-44">
                    <DropdownMenuItem :disabled="running[t.id]" @select="runNow(t, true)"><FlaskConical />试运行（不改 DNS）</DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem @select="router.push(`/tasks/${t.id}`)"><Pencil />编辑</DropdownMenuItem>
                    <DropdownMenuItem @select="clone(t)"><Copy />克隆</DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem variant="destructive" @select="remove(t)"><Trash2 />删除</DropdownMenuItem>
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
