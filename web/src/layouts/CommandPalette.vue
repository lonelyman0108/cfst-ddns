<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useMagicKeys, whenever } from '@vueuse/core'
import { toast } from 'vue-sonner'
import { FileInput, ListChecks, Monitor, Moon, Play, Plus, Rocket, Sun } from '@lucide/vue'
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
import { HELP_SECTIONS } from '@/components/help/sections'
import { LOCALES } from '@/i18n'
import { NAV } from './nav'

const open = defineModel<boolean>({ default: false })
const router = useRouter()
const theme = useThemeStore()
const tasks = ref<Task[]>([])
const { t } = useI18n()

// 列表按文本内容过滤：把各语言的译文作为隐藏关键词附在条目里，任何语言下都能搜到
const kw = (key: string) => [...new Set(LOCALES.map((l) => t(key, {}, { locale: l.value })))].join(' ')

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
      .then((list) => (tasks.value = list ?? []))
      .catch(() => {})
  }
})

function go(path: string) {
  open.value = false
  router.push(path)
}

async function run(task: Task) {
  open.value = false
  try {
    const { runId } = await tasksApi.run(task.id)
    toast.success(t('layout.palette.queued', { name: task.name }))
    router.push(`/runs/${runId}`)
  } catch (e) {
    if (errorStatus(e) === 409) {
      const active = await runsApi.active().catch(() => [])
      const r = active.find((a) => a.taskId === task.id)
      if (r) router.push(`/runs/${r.id}`)
    }
  }
}
</script>

<template>
  <CommandDialog v-model:open="open" :title="t('layout.palette.title')" :description="t('layout.palette.description')">
    <CommandInput :placeholder="t('layout.palette.placeholder')" />
    <CommandList>
      <CommandEmpty>{{ t('layout.palette.empty') }}</CommandEmpty>
      <CommandGroup :heading="t('layout.palette.pages')">
        <CommandItem v-for="n in NAV" :key="n.path" :value="`page:${n.title}`" @select="go(n.path)">
          <component :is="n.icon" />{{ t(n.title) }}
          <span class="hidden">{{ kw(n.title) }}</span>
        </CommandItem>
      </CommandGroup>
      <CommandSeparator />
      <CommandGroup v-if="tasks.length" :heading="t('layout.palette.tasks')">
        <CommandItem v-for="task in tasks" :key="`edit-${task.id}`" :value="`task:${task.name}:${task.id}`" @select="go(`/tasks/${task.id}`)">
          <ListChecks />{{ task.name }}
          <CommandShortcut>{{ t('common.edit') }}</CommandShortcut>
        </CommandItem>
      </CommandGroup>
      <CommandGroup v-if="tasks.length" :heading="t('layout.palette.runNow')">
        <CommandItem v-for="task in tasks" :key="`run-${task.id}`" :value="`run:${task.name}:${task.id}`" @select="run(task)">
          <Play />{{ t('layout.palette.runTask', { name: task.name }) }}
        </CommandItem>
      </CommandGroup>
      <CommandSeparator />
      <CommandGroup :heading="t('layout.palette.help')">
        <CommandItem v-for="h in HELP_SECTIONS" :key="h.id" :value="`help:${h.id}`" @select="go(`/help#${h.id}`)">
          <component :is="h.icon" />{{ t(h.title) }}
          <CommandShortcut>{{ t('layout.palette.help') }}</CommandShortcut>
          <span class="hidden">{{ kw('layout.palette.help') }} {{ kw(h.title) }} {{ h.keywords }}</span>
        </CommandItem>
      </CommandGroup>
      <CommandSeparator />
      <CommandGroup :heading="t('layout.palette.actions')">
        <CommandItem value="action:new-task" @select="go('/tasks/new')">
          <Plus />{{ t('layout.palette.newTask') }}<span class="hidden">{{ kw('layout.palette.newTask') }}</span>
        </CommandItem>
        <CommandItem value="action:welcome" @select="go('/welcome')">
          <Rocket />{{ t('layout.palette.quickStart') }}<span class="hidden">{{ kw('layout.palette.quickStart') }} 引导 向导 welcome</span>
        </CommandItem>
        <CommandItem value="action:import-legacy" @select="go('/import/legacy')">
          <FileInput />{{ t('layout.palette.importLegacy') }}<span class="hidden">{{ kw('layout.palette.importLegacy') }} 旧版 迁移 legacy</span>
        </CommandItem>
        <CommandItem value="action:theme-light" @select="theme.setMode('light')">
          <Sun />{{ t('layout.palette.lightTheme') }}<span class="hidden">{{ kw('layout.palette.lightTheme') }} light</span>
        </CommandItem>
        <CommandItem value="action:theme-dark" @select="theme.setMode('dark')">
          <Moon />{{ t('layout.palette.darkTheme') }}<span class="hidden">{{ kw('layout.palette.darkTheme') }} dark</span>
        </CommandItem>
        <CommandItem value="action:theme-system" @select="theme.setMode('system')">
          <Monitor />{{ t('layout.palette.systemTheme') }}<span class="hidden">{{ kw('layout.palette.systemTheme') }} system</span>
        </CommandItem>
      </CommandGroup>
    </CommandList>
  </CommandDialog>
</template>
