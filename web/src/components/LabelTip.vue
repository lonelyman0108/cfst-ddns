<script setup lang="ts">
import { CircleHelp } from '@lucide/vue'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

defineProps<{ tip?: string }>()
</script>

<template>
  <Tooltip v-if="tip">
    <TooltipTrigger as-child>
      <button
        type="button"
        tabindex="-1"
        class="text-muted-foreground hover:text-foreground inline-flex cursor-help align-middle transition-colors"
      >
        <CircleHelp class="size-3.5" />
      </button>
    </TooltipTrigger>
    <TooltipContent class="max-w-xs leading-relaxed whitespace-pre-line">{{ tip }}</TooltipContent>
  </Tooltip>
</template>
