<script setup lang="ts">
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'
import LabelTip from './LabelTip.vue'

const props = defineProps<{
  label?: string
  tip?: string
  help?: string
  error?: string
  required?: boolean
  for?: string
  class?: string
}>()
</script>

<template>
  <div :class="cn('grid content-start gap-1.5', props.class)">
    <div v-if="label || $slots.label" class="flex min-h-5 items-center gap-1.5">
      <Label :for="props.for" class="text-label text-muted-foreground">
        <slot name="label">{{ label }}</slot>
        <span v-if="required" class="text-destructive -ml-1">*</span>
      </Label>
      <LabelTip :tip="tip" />
      <div class="ml-auto"><slot name="extra" /></div>
    </div>
    <slot />
    <p v-if="error" class="text-destructive -mt-0.5 text-xs">{{ error }}</p>
    <p v-else-if="help || $slots.help" class="text-muted-foreground -mt-0.5 text-xs">
      <slot name="help">{{ help }}</slot>
    </p>
  </div>
</template>
