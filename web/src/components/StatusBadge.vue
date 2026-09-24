<script setup lang="ts">
import { computed } from 'vue'
import type { RunStatus } from '@/api/types'
import { runStatusMeta } from '@/utils/format'
import { cn } from '@/lib/utils'

const props = defineProps<{ status?: RunStatus | null; class?: string }>()

const tones: Record<RunStatus, { dot: string; badge: string }> = {
  queued: { dot: 'bg-muted-foreground', badge: 'text-muted-foreground' },
  running: { dot: 'bg-info', badge: 'text-info border-info/30 bg-info/10' },
  success: { dot: 'bg-success', badge: 'text-success border-success/30 bg-success/10' },
  partial: { dot: 'bg-warning', badge: 'text-warning border-warning/30 bg-warning/10' },
  failed: { dot: 'bg-destructive', badge: 'text-destructive border-destructive/30 bg-destructive/10' },
  canceled: { dot: 'bg-muted-foreground/60', badge: 'text-muted-foreground' },
}

const meta = computed(() => (props.status ? runStatusMeta[props.status] : null))
const tone = computed(() => (props.status ? tones[props.status] : null))
const pulsing = computed(() => props.status === 'running' || props.status === 'queued')
</script>

<template>
  <span
    v-if="meta && tone"
    :class="cn('inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium whitespace-nowrap', tone.badge, props.class)"
  >
    <span class="relative flex size-1.5">
      <span v-if="pulsing" :class="cn('absolute inline-flex size-full animate-ping rounded-full opacity-75', tone.dot)" />
      <span :class="cn('relative inline-flex size-1.5 rounded-full', tone.dot)" />
    </span>
    {{ meta.label }}
  </span>
  <span v-else class="text-muted-foreground text-xs">-</span>
</template>
