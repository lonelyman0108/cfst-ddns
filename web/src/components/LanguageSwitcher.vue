<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Languages } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { LOCALES, setLocale, type Locale } from '@/i18n'
import { useMetaStore } from '@/stores/meta'

const { t, locale } = useI18n()
const meta = useMetaStore()

const model = computed({
  get: () => locale.value,
  set: (v: string) => {
    if (v === locale.value) return
    setLocale(v as Locale)
    // 服务商 / 渠道的表单字段由后端按语言返回，切换后重新拉取
    meta.loadProviders(true).catch(() => {})
    meta.loadNotifiers(true).catch(() => {})
  },
})
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <Button variant="ghost" size="icon" class="size-8" :aria-label="t('common.language')" :title="t('common.language')">
        <Languages />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" class="w-36">
      <DropdownMenuRadioGroup v-model="model">
        <DropdownMenuRadioItem v-for="l in LOCALES" :key="l.value" :value="l.value" :lang="l.value">{{ l.label }}</DropdownMenuRadioItem>
      </DropdownMenuRadioGroup>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
