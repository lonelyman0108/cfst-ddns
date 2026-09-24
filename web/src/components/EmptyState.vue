<script setup lang="ts">
import type { Component } from 'vue'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'

/** compact：卡片内空状态（py-6、小图标）；默认为页面级空状态（py-10） */
defineProps<{ icon?: Component; title: string; description?: string; compact?: boolean }>()
</script>

<template>
  <Empty :class="compact ? 'gap-3 py-6' : ''">
    <EmptyHeader :class="compact ? 'gap-1' : ''">
      <EmptyMedia v-if="icon" variant="icon" :class="compact ? 'mb-1 size-8 [&_svg:not([class*=size-])]:size-4' : ''">
        <component :is="icon" />
      </EmptyMedia>
      <EmptyTitle :class="compact ? 'text-sm' : ''">{{ title }}</EmptyTitle>
      <EmptyDescription v-if="description" :class="compact ? 'text-xs' : ''">{{ description }}</EmptyDescription>
    </EmptyHeader>
    <EmptyContent v-if="$slots.default">
      <slot />
    </EmptyContent>
  </Empty>
</template>
