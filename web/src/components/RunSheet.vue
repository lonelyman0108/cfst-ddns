<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { SquareArrowOutUpRight } from '@lucide/vue'
import type { RunSummary } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import RunLogViewer from './RunLogViewer.vue'

const props = defineProps<{ runId: number | null }>()
const open = defineModel<boolean>({ default: false })
const emit = defineEmits<{ (e: 'done', s: RunSummary): void }>()

const router = useRouter()
const title = ref('')

function onStatus(s: RunSummary) {
  title.value = `${s.taskName || '任务'} · 执行 #${s.id}`
}

function openPage() {
  if (!props.runId) return
  open.value = false
  router.push(`/runs/${props.runId}`)
}
</script>

<template>
  <Sheet v-model:open="open">
    <SheetContent class="bg-background w-full gap-0 overflow-y-auto p-0 sm:max-w-4xl">
      <SheetHeader class="bg-background/95 sticky top-0 z-10 border-b px-5 py-4 backdrop-blur">
        <div class="flex items-center gap-2 pr-8">
          <SheetTitle class="truncate">{{ title || '执行详情' }}</SheetTitle>
          <Button variant="ghost" size="sm" class="ml-auto" @click="openPage">
            <SquareArrowOutUpRight />在页面中打开
          </Button>
        </div>
        <SheetDescription class="sr-only">执行实时日志与结果</SheetDescription>
      </SheetHeader>
      <div class="p-4">
        <RunLogViewer
          v-if="runId && open"
          :run-id="runId"
          log-height="max(320px, calc(100vh - 420px))"
          @status="onStatus"
          @done="emit('done', $event)"
        />
      </div>
    </SheetContent>
  </Sheet>
</template>
