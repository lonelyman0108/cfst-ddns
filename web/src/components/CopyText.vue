<script setup lang="ts">
import { Copy } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { copyText } from '@/utils/format'

defineProps<{ text: string }>()
const { t } = useI18n()
</script>

<template>
  <span class="group/copy inline-flex max-w-full items-center gap-1 font-mono text-code">
    <span class="min-w-0 break-all">{{ text }}</span>
    <button
      v-if="text"
      type="button"
      :title="t('common.copy')"
      class="text-muted-foreground hover:text-foreground hover:bg-accent shrink-0 rounded p-0.5 opacity-0 transition-opacity group-hover/copy:opacity-100 focus-visible:opacity-100"
      @click.stop="copyText(text)"
    >
      <Copy class="size-3.5" />
    </button>
  </span>
</template>
