<script setup lang="ts">
/** 设置行：标签与说明在左、控件（通常为 Switch）在右；放在 divide-y 容器中使用 */
defineProps<{ label: string; description?: string; flag?: string; for?: string }>()
</script>

<template>
  <div class="flex min-h-11 items-center justify-between gap-4 px-3 py-2">
    <div class="min-w-0">
      <div class="flex items-center gap-1.5">
        <label :for="$props.for" class="cursor-pointer text-sm font-medium">{{ label }}</label>
        <code v-if="flag" class="text-muted-foreground/80 bg-muted rounded px-1 font-mono text-[11px] leading-4">{{ flag }}</code>
      </div>
      <p v-if="description" class="text-muted-foreground text-xs">{{ description }}</p>
    </div>
    <div class="shrink-0"><slot /></div>
  </div>
</template>
