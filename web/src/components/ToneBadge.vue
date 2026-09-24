<script setup lang="ts">
import { cn } from '@/lib/utils'
import type { Tone } from '@/utils/format'

const props = withDefaults(defineProps<{ tone?: Tone; class?: string }>(), { tone: 'neutral' })

const cls: Record<Tone, string> = {
  primary: 'text-primary border-primary/30 bg-primary/10',
  success: 'text-success border-success/30 bg-success/10',
  warning: 'text-warning border-warning/30 bg-warning/10',
  danger: 'text-destructive border-destructive/30 bg-destructive/10',
  info: 'text-info border-info/30 bg-info/10',
  neutral: 'text-muted-foreground border-border bg-muted/50',
}
</script>

<template>
  <span
    :class="cn('inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-xs font-medium whitespace-nowrap', cls[props.tone], props.class)"
  >
    <slot />
  </span>
</template>
