<script setup lang="ts">
import { computed } from 'vue'
import { NumberField, NumberFieldContent, NumberFieldInput } from '@/components/ui/number-field'
import { cn } from '@/lib/utils'

/** 轻量数字输入：无 +/- 按钮，支持方向键增减，单位显示在右侧 */
const props = withDefaults(
  defineProps<{
    min?: number
    max?: number
    step?: number
    decimals?: number
    disabled?: boolean
    id?: string
    suffix?: string
    class?: string
  }>(),
  { step: 1, decimals: 0 },
)
const model = defineModel<number | undefined>()

const fmt = computed(() => ({ useGrouping: false, maximumFractionDigits: props.decimals, minimumFractionDigits: 0 }))

function onUpdate(v: number) {
  model.value = Number.isFinite(v) ? v : undefined
}
</script>

<template>
  <NumberField
    :id="id"
    :model-value="model"
    :min="min"
    :max="max"
    :step="step"
    :disabled="disabled"
    :format-options="fmt"
    :class="props.class"
    @update:model-value="onUpdate"
  >
    <NumberFieldContent>
      <NumberFieldInput :class="cn('px-3 text-left tabular-nums', suffix && 'pr-12')" />
      <span
        v-if="suffix"
        class="text-muted-foreground pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs"
      >
        {{ suffix }}
      </span>
    </NumberFieldContent>
  </NumberField>
</template>
