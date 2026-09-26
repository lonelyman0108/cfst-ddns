<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, Check, Sparkles, X } from '@lucide/vue'
import { onboardingApi, settingsApi } from '@/api'
import type { OnboardingState } from '@/api/types-p9'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { cn } from '@/lib/utils'
import { ONBOARDING_STEPS } from './steps'

const { t } = useI18n()
const state = ref<OnboardingState | null>(null)
const dismissed = ref(true)
const dismissing = ref(false)

const doneCount = computed(() => (state.value ? ONBOARDING_STEPS.filter((s) => state.value![s.key]).length : 0))
const allDone = computed(() => doneCount.value === ONBOARDING_STEPS.length)
const visible = computed(() => !!state.value && !dismissed.value && !allDone.value)

onMounted(async () => {
  const s = await settingsApi.get().catch(() => null)
  if (!s || s.onboardingDismissed) return
  dismissed.value = false
  state.value = await onboardingApi.state()
})

async function dismiss() {
  dismissing.value = true
  try {
    await settingsApi.update({ onboardingDismissed: true })
    dismissed.value = true
  } catch {
    /* 已提示 */
  } finally {
    dismissing.value = false
  }
}

defineExpose({ reload: async () => (state.value = await onboardingApi.state()) })
</script>

<template>
  <Card v-if="visible" class="border-primary/30 gap-3">
    <CardHeader>
      <CardTitle class="flex items-center gap-2"><Sparkles class="text-primary size-4" />{{ t('onboarding.checklist.title') }}</CardTitle>
      <CardDescription>{{ t('onboarding.checklist.description') }}</CardDescription>
      <CardAction>
        <Button variant="ghost" size="icon" class="-mr-2 size-8" :title="t('onboarding.checklist.dismiss')" :disabled="dismissing" @click="dismiss"><X /></Button>
      </CardAction>
    </CardHeader>
    <CardContent class="grid grid-cols-1 gap-3">
      <div class="flex items-center gap-3">
        <Progress :model-value="(doneCount / ONBOARDING_STEPS.length) * 100" class="h-1.5" />
        <span class="text-muted-foreground shrink-0 text-xs tabular-nums">{{ doneCount }}/{{ ONBOARDING_STEPS.length }}</span>
      </div>
      <ul class="grid grid-cols-1 gap-1 sm:grid-cols-2 lg:grid-cols-5">
        <li v-for="s in ONBOARDING_STEPS" :key="s.key">
          <router-link
            :to="{ path: '/welcome', query: { step: s.key } }"
            :class="
              cn(
                'hover:bg-accent/60 group flex min-h-11 items-center gap-2.5 rounded-md px-2 py-1.5 text-sm transition-colors',
                state?.[s.key] ? 'text-muted-foreground' : '',
              )
            "
          >
            <span
              :class="
                cn(
                  'flex size-5 shrink-0 items-center justify-center rounded-full border',
                  state?.[s.key] ? 'bg-primary border-primary text-primary-foreground' : '',
                )
              "
            >
              <Check v-if="state?.[s.key]" class="size-3" />
            </span>
            <span :class="cn('min-w-0 flex-1 truncate', state?.[s.key] ? 'line-through' : 'font-medium')">
              {{ t(s.title) }}<span v-if="s.optional" class="text-muted-foreground font-normal">{{ t('onboarding.checklist.optional') }}</span>
            </span>
            <ArrowRight v-if="!state?.[s.key]" class="text-muted-foreground size-3.5 transition-transform group-hover:translate-x-0.5" />
          </router-link>
        </li>
      </ul>
    </CardContent>
  </Card>
</template>
