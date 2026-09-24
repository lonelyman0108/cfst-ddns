<script setup lang="ts">
import { ChevronRight } from '@lucide/vue'
import type { TypeMeta } from '@/api/types'

defineProps<{ types: TypeMeta[] }>()
const emit = defineEmits<{ (e: 'pick', t: TypeMeta): void }>()
</script>

<template>
  <div class="grid gap-2 sm:grid-cols-2">
    <button
      v-for="t in types"
      :key="t.type"
      type="button"
      class="hover:border-primary/50 hover:bg-primary/5 group flex items-start gap-3 rounded-lg border p-3 text-left transition-colors"
      @click="emit('pick', t)"
    >
      <div class="bg-muted text-foreground flex size-9 shrink-0 items-center justify-center rounded-md text-sm font-semibold">
        {{ t.name.slice(0, 1) }}
      </div>
      <div class="min-w-0 flex-1">
        <div class="text-sm font-medium">{{ t.name }}</div>
        <div class="text-muted-foreground mt-0.5 line-clamp-2 text-xs leading-relaxed">{{ t.description || t.type }}</div>
      </div>
      <ChevronRight class="text-muted-foreground mt-2 size-4 shrink-0 transition-transform group-hover:translate-x-0.5" />
    </button>
    <p v-if="!types.length" class="text-muted-foreground col-span-full py-6 text-center text-sm">无法获取类型列表</p>
  </div>
</template>
