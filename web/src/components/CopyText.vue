<script setup lang="ts">
import { Copy } from '@lucide/vue'
import { copyText } from '@/utils/format'

defineProps<{ text: string }>()
</script>

<template>
  <span class="group/copy inline-flex max-w-full items-center gap-1 font-mono text-code">
    <span class="truncate">{{ text }}</span>
    <button
      v-if="text"
      type="button"
      title="复制"
      class="text-muted-foreground hover:text-foreground hover:bg-accent rounded p-0.5 opacity-0 transition-opacity group-hover/copy:opacity-100 focus-visible:opacity-100"
      @click.stop="copyText(text)"
    >
      <Copy class="size-3.5" />
    </button>
  </span>
</template>
