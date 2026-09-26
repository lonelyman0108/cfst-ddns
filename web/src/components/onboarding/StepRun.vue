<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { FlaskConical, Loader2, Play, Rocket } from '@lucide/vue'
import { errorStatus, runsApi, tasksApi } from '@/api'
import type { RunSummary, Task } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import FormItem from '@/components/FormItem.vue'
import RunLogViewer from '@/components/RunLogViewer.vue'
import { cn } from '@/lib/utils'

const emit = defineEmits<{ (e: 'changed'): void; (e: 'finished', s: RunSummary): void }>()

const tasks = ref<Task[] | null>(null)
const taskId = ref(0)
const dryRun = ref(true)
const starting = ref(false)
const runId = ref<number | null>(null)
const finished = ref<RunSummary | null>(null)

const MODES = [
  { dry: true, icon: FlaskConical, title: '试运行', description: '只测速，不修改 DNS、不发通知' },
  { dry: false, icon: Rocket, title: '正式执行', description: '测速并写入 DNS 记录' },
]

const task = computed(() => tasks.value?.find((t) => t.id === taskId.value))

onMounted(async () => {
  tasks.value = (await tasksApi.list().catch(() => [])) ?? []
  const newest = [...tasks.value].sort((a, b) => b.id - a.id)[0]
  if (newest) taskId.value = newest.id
})

async function start() {
  if (!taskId.value) return
  starting.value = true
  finished.value = null
  try {
    const r = await tasksApi.run(taskId.value, { dryRun: dryRun.value })
    runId.value = r.runId
  } catch (e) {
    if (errorStatus(e) === 409) {
      const active = await runsApi.active().catch(() => [])
      const cur = active.find((a) => a.taskId === taskId.value)
      if (cur) runId.value = cur.id
    }
  } finally {
    starting.value = false
  }
}

function onDone(s: RunSummary) {
  finished.value = s
  emit('finished', s)
  emit('changed')
}
</script>

<template>
  <div class="grid grid-cols-[minmax(0,1fr)] gap-4">
    <Skeleton v-if="!tasks" class="h-32" />
    <p v-else-if="!tasks.length" class="text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm">
      需要先创建一个任务
    </p>
    <template v-else>
      <div v-if="!runId" class="grid gap-4">
        <FormItem label="任务">
          <Select :model-value="taskId || undefined" @update:model-value="taskId = Number($event)">
            <SelectTrigger class="w-full"><SelectValue placeholder="选择任务" /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="t in tasks" :key="t.id" :value="t.id">{{ t.name }}</SelectItem>
            </SelectContent>
          </Select>
        </FormItem>
        <div class="grid gap-2 sm:grid-cols-2">
          <button
            v-for="m in MODES"
            :key="String(m.dry)"
            type="button"
            :class="
              cn(
                'flex items-start gap-3 rounded-lg border p-3 text-left transition-colors',
                dryRun === m.dry ? 'border-primary bg-primary/5' : 'hover:bg-accent/50',
              )
            "
            @click="dryRun = m.dry"
          >
            <component :is="m.icon" class="text-muted-foreground mt-0.5 size-4 shrink-0" />
            <span>
              <span class="block text-sm font-medium">{{ m.title }}</span>
              <span class="text-muted-foreground mt-0.5 block text-xs">{{ m.description }}</span>
            </span>
          </button>
        </div>
        <p class="text-muted-foreground text-xs">测速通常需要 1–3 分钟，期间可以离开本页，执行不会中断。</p>
        <div>
          <Button :disabled="!task || starting" @click="start">
            <Loader2 v-if="starting" class="animate-spin" /><Play v-else />{{ dryRun ? '开始试运行' : '开始执行' }}
          </Button>
        </div>
      </div>

      <template v-else>
        <RunLogViewer :run-id="runId" log-height="360px" @done="onDone" />
        <div v-if="finished">
          <Button variant="outline" size="sm" @click="runId = null"><Play />再执行一次</Button>
        </div>
      </template>
    </template>
  </div>
</template>
