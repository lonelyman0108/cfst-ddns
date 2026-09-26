<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
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

const { t } = useI18n()
const tasks = ref<Task[] | null>(null)
const taskId = ref(0)
const dryRun = ref(true)
const starting = ref(false)
const runId = ref<number | null>(null)
const finished = ref<RunSummary | null>(null)

// 文案见 onboarding.run.modes.<key>
const MODES = [
  { dry: true, key: 'dry', icon: FlaskConical },
  { dry: false, key: 'real', icon: Rocket },
]

const task = computed(() => tasks.value?.find((x) => x.id === taskId.value))

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
      {{ t('onboarding.run.needTask') }}
    </p>
    <template v-else>
      <div v-if="!runId" class="grid grid-cols-1 gap-4">
        <FormItem :label="t('onboarding.run.task')">
          <Select :model-value="taskId || undefined" @update:model-value="taskId = Number($event)">
            <SelectTrigger class="w-full"><SelectValue :placeholder="t('onboarding.run.selectTask')" /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="x in tasks" :key="x.id" :value="x.id">{{ x.name }}</SelectItem>
            </SelectContent>
          </Select>
        </FormItem>
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
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
              <span class="block text-sm font-medium">{{ t(`onboarding.run.modes.${m.key}.title`) }}</span>
              <span class="text-muted-foreground mt-0.5 block text-xs">{{ t(`onboarding.run.modes.${m.key}.description`) }}</span>
            </span>
          </button>
        </div>
        <p class="text-muted-foreground text-xs">{{ t('onboarding.run.hint') }}</p>
        <div>
          <Button :disabled="!task || starting" @click="start">
            <Loader2 v-if="starting" class="animate-spin" /><Play v-else />{{ dryRun ? t('onboarding.run.startDry') : t('onboarding.run.start') }}
          </Button>
        </div>
      </div>

      <template v-else>
        <RunLogViewer :run-id="runId" log-height="360px" @done="onDone" />
        <div v-if="finished">
          <Button variant="outline" size="sm" @click="runId = null"><Play />{{ t('onboarding.run.again') }}</Button>
        </div>
      </template>
    </template>
  </div>
</template>
