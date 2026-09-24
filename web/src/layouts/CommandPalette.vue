<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useMagicKeys, whenever } from '@vueuse/core'
import { toast } from 'vue-sonner'
import { ListChecks, Monitor, Moon, Play, Plus, Sun } from '@lucide/vue'
import { errorStatus, runsApi, tasksApi } from '@/api'
import type { Task } from '@/api/types'
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  CommandShortcut,
} from '@/components/ui/command'
import { useThemeStore } from '@/stores/theme'
import { NAV } from './nav'

const open = defineModel<boolean>({ default: false })
const router = useRouter()
const theme = useThemeStore()
const tasks = ref<Task[]>([])

const keys = useMagicKeys({
  passive: false,
  onEventFired(e) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k' && e.type === 'keydown') e.preventDefault()
  },
})
whenever(
  () => !!(keys['Ctrl+K']?.value || keys['Meta+K']?.value),
  () => (open.value = !open.value),
)

watch(open, (v) => {
  if (v) {
    tasksApi
      .list()
      .then((t) => (tasks.value = t ?? []))
      .catch(() => {})
  }
})

function go(path: string) {
  open.value = false
  router.push(path)
}

async function run(t: Task) {
  open.value = false
  try {
    const { runId } = await tasksApi.run(t.id)
    toast.success(`「${t.name}」已加入执行队列`)
    router.push(`/runs/${runId}`)
  } catch (e) {
    if (errorStatus(e) === 409) {
      const active = await runsApi.active().catch(() => [])
      const r = active.find((a) => a.taskId === t.id)
      if (r) router.push(`/runs/${r.id}`)
    }
  }
}
</script>

<template>
  <CommandDialog v-model:open="open" title="命令面板" description="跳转页面、搜索任务或立即执行">
    <CommandInput placeholder="输入页面、任务名或命令…" />
    <CommandList>
      <CommandEmpty>没有匹配的结果</CommandEmpty>
      <CommandGroup heading="页面">
        <CommandItem v-for="n in NAV" :key="n.path" :value="`page:${n.title}`" @select="go(n.path)">
          <component :is="n.icon" />{{ n.title }}
        </CommandItem>
      </CommandGroup>
      <CommandSeparator />
      <CommandGroup v-if="tasks.length" heading="任务">
        <CommandItem v-for="t in tasks" :key="`edit-${t.id}`" :value="`task:${t.name}:${t.id}`" @select="go(`/tasks/${t.id}`)">
          <ListChecks />{{ t.name }}
          <CommandShortcut>编辑</CommandShortcut>
        </CommandItem>
      </CommandGroup>
      <CommandGroup v-if="tasks.length" heading="立即执行">
        <CommandItem v-for="t in tasks" :key="`run-${t.id}`" :value="`run:执行 ${t.name}:${t.id}`" @select="run(t)">
          <Play />执行「{{ t.name }}」
        </CommandItem>
      </CommandGroup>
      <CommandSeparator />
      <CommandGroup heading="操作">
        <CommandItem value="action:新建任务" @select="go('/tasks/new')"><Plus />新建任务</CommandItem>
        <CommandItem value="action:亮色主题 light" @select="theme.setMode('light')"><Sun />亮色主题</CommandItem>
        <CommandItem value="action:暗色主题 dark" @select="theme.setMode('dark')"><Moon />暗色主题</CommandItem>
        <CommandItem value="action:跟随系统主题 system" @select="theme.setMode('system')"><Monitor />跟随系统主题</CommandItem>
      </CommandGroup>
    </CommandList>
  </CommandDialog>
</template>
