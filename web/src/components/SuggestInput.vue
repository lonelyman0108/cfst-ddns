<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, ChevronDown, Loader2 } from '@lucide/vue'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

/** 可自由输入、带候选下拉的输入框（候选加载失败时即为普通输入框） */
const props = defineProps<{
  options: string[]
  loading?: boolean
  placeholder?: string
  class?: string
}>()
const emit = defineEmits<{ (e: 'open'): void }>()
const model = defineModel<string>({ default: '' })

const open = ref(false)
const active = ref(-1)

const filtered = computed(() => {
  const q = model.value.trim().toLowerCase()
  const list = props.options ?? []
  if (!q || list.includes(model.value.trim())) return list
  return list.filter((o) => o.toLowerCase().includes(q))
})

function show() {
  open.value = true
  active.value = -1
  emit('open')
}

function pick(v: string) {
  model.value = v
  open.value = false
}

function onKey(e: KeyboardEvent) {
  if (!open.value && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
    show()
    return
  }
  if (!filtered.value.length) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    active.value = (active.value + 1) % filtered.value.length
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    active.value = (active.value - 1 + filtered.value.length) % filtered.value.length
  } else if (e.key === 'Enter' && active.value >= 0) {
    e.preventDefault()
    pick(filtered.value[active.value])
  } else if (e.key === 'Escape') {
    open.value = false
  }
}
</script>

<template>
  <div :class="cn('relative', props.class)">
    <Input
      v-model="model"
      :placeholder="placeholder"
      class="pr-8"
      autocomplete="off"
      @focus="show"
      @blur="open = false"
      @keydown="onKey"
    />
    <span class="text-muted-foreground pointer-events-none absolute inset-y-0 right-2.5 flex items-center">
      <Loader2 v-if="loading" class="size-4 animate-spin" />
      <ChevronDown v-else-if="options.length" class="size-4 opacity-60" />
    </span>
    <div
      v-if="open && filtered.length"
      class="bg-popover text-popover-foreground animate-in fade-in-0 zoom-in-95 absolute top-full z-50 mt-1 max-h-60 w-full overflow-auto rounded-md border p-1 shadow-md"
    >
      <button
        v-for="(o, i) in filtered"
        :key="o"
        type="button"
        :class="
          cn(
            'hover:bg-accent flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm',
            i === active && 'bg-accent',
          )
        "
        @mousedown.prevent="pick(o)"
      >
        <span class="flex-1 truncate" :title="o">{{ o }}</span>
        <Check v-if="o === model" class="size-4" />
      </button>
    </div>
  </div>
</template>
