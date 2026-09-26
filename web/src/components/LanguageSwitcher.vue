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
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { LOCALES, browserLocale, localePreference, setLocalePreference, type LocalePreference } from '@/i18n'
import { useMetaStore } from '@/stores/meta'

const { t } = useI18n()
const meta = useMetaStore()

const model = computed({
  get: () => localePreference.value,
  set: (v: string) => {
    if (v === localePreference.value) return
    setLocalePreference(v as LocalePreference)
    // 服务商 / 渠道的表单字段由后端按语言返回，切换后重新拉取
    meta.loadProviders(true).catch(() => {})
    meta.loadNotifiers(true).catch(() => {})
  },
})

const browserLabel = computed(() => LOCALES.find((l) => l.value === browserLocale.value)?.label ?? '')
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <Button variant="ghost" size="icon" class="size-8" :aria-label="t('common.language')" :title="t('common.language')">
        <Languages />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" class="min-w-44">
      <DropdownMenuRadioGroup v-model="model">
        <DropdownMenuRadioItem value="auto">{{ t('common.followBrowser', { lang: browserLabel }) }}</DropdownMenuRadioItem>
        <DropdownMenuSeparator />
        <DropdownMenuRadioItem v-for="l in LOCALES" :key="l.value" :value="l.value" :lang="l.value">{{ l.label }}</DropdownMenuRadioItem>
      </DropdownMenuRadioGroup>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
