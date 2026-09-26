<script setup lang="ts">
import { computed } from 'vue'
import { Mail, Webhook } from '@lucide/vue'
import { brandImg, brandSvg, notifierBrands, providerBrands } from '@/assets/brands'
import { cn } from '@/lib/utils'

// 服务商 / 通知渠道的品牌图标方块；尺寸通过 class 调整（默认 size-8）
const props = defineProps<{
  kind: 'provider' | 'notifier'
  type: string
  /** 未知类型时用于取首字母的名称 */
  name?: string
  class?: string
}>()

const brand = computed(() => (props.kind === 'provider' ? providerBrands : notifierBrands)[props.type])
const svg = computed(() => (brand.value?.svg ? brandSvg(brand.value.svg) : undefined))
const img = computed(() => (brand.value?.img ? brandImg(brand.value.img) : undefined))
const lucide = computed(() => (brand.value?.lucide === 'mail' ? Mail : brand.value?.lucide === 'webhook' ? Webhook : undefined))
const initial = computed(() => brand.value?.initial ?? (props.name || props.type || '?').slice(0, 1).toUpperCase())
const vars = computed(() =>
  brand.value ? { '--brand': brand.value.color, '--brand-dark': brand.value.darkColor ?? brand.value.color } : undefined,
)
</script>

<template>
  <span
    v-if="img && brand?.fill"
    :class="cn('inline-flex size-8 shrink-0 overflow-hidden rounded-md border', props.class)"
  >
    <img :src="img" alt="" class="size-full object-cover" draggable="false" />
  </span>
  <span
    v-else-if="svg || lucide || img"
    :style="vars"
    :class="
      cn(
        'inline-flex size-8 shrink-0 items-center justify-center rounded-md border bg-[color-mix(in_oklab,var(--brand)_8%,transparent)] text-(--brand) dark:bg-[color-mix(in_oklab,var(--brand-dark)_14%,transparent)] dark:text-(--brand-dark)',
        props.class,
      )
    "
  >
    <img v-if="img" :src="img" alt="" class="size-[62%] object-contain" draggable="false" />
    <component :is="lucide" v-else-if="lucide" class="size-[58%] text-current" />
    <span v-else class="contents" v-html="svg" />
  </span>
  <span
    v-else
    :style="vars"
    :class="
      cn(
        'inline-flex size-8 shrink-0 items-center justify-center rounded-md font-semibold text-white select-none [container-type:size]',
        brand ? 'bg-(--brand)' : 'bg-muted text-muted-foreground border',
        props.class,
      )
    "
  >
    <span class="text-[45cqh] leading-none">{{ initial }}</span>
  </span>
</template>
