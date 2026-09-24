<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { cn } from "@/lib/utils"

const props = defineProps<{
  class?: HTMLAttributes["class"]
}>()
</script>

<template>
  <th
    data-slot="table-head"
    :class="cn('text-muted-foreground text-label h-9 px-2 text-left align-middle font-medium whitespace-nowrap [&:has([role=checkbox])]:pr-0 *:[[role=checkbox]]:translate-y-0.5', props.class)"
  >
    <slot />
  </th>
</template>
