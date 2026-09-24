<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>

<template>
  <div class="flex flex-wrap items-end justify-between gap-3">
    <div class="min-w-0">
      <div class="flex items-center gap-2">
        <slot name="before" />
        <h1 class="page-title truncate">{{ title }}</h1>
        <slot name="after" />
      </div>
      <p v-if="description" class="text-muted-foreground mt-0.5">{{ description }}</p>
    </div>
    <div class="flex flex-wrap items-center gap-2"><slot /></div>
  </div>
</template>
