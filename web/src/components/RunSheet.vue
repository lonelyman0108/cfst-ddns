<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { SquareArrowOutUpRight } from '@lucide/vue'
import type { RunSummary } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import RunLogViewer from './RunLogViewer.vue'

const props = defineProps<{ runId: number | null }>()
const open = defineModel<boolean>({ default: false })
const emit = defineEmits<{ (e: 'done', s: RunSummary): void }>()

const { t } = useI18n()
const router = useRouter()
const current = ref<RunSummary | null>(null)
const title = computed(() =>
  current.value ? t('runs.sheet.title', { task: current.value.taskName || t('runs.sheet.task'), id: current.value.id }) : '',
)

function onStatus(s: RunSummary) {
  current.value = s
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
          <SheetTitle class="truncate">{{ title || t('runs.detail.title') }}</SheetTitle>
          <Button variant="ghost" size="sm" class="ml-auto" @click="openPage">
            <SquareArrowOutUpRight />{{ t('common.openInPage') }}
          </Button>
        </div>
        <SheetDescription class="sr-only">{{ t('runs.sheet.srDesc') }}</SheetDescription>
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
