<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ArrowRight, Check, LayoutDashboard } from '@lucide/vue'
import { onboardingApi } from '@/api'
import type { OnboardingState } from '@/api/types-p9'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import StepAccount from '@/components/onboarding/StepAccount.vue'
import StepCfst from '@/components/onboarding/StepCfst.vue'
import StepNotifier from '@/components/onboarding/StepNotifier.vue'
import StepRun from '@/components/onboarding/StepRun.vue'
import StepTask from '@/components/onboarding/StepTask.vue'
import { ONBOARDING_STEPS, firstPendingStep, stepIndex } from '@/components/onboarding/steps'
import { cn } from '@/lib/utils'

const route = useRoute()
const router = useRouter()

const state = ref<OnboardingState | null>(null)
const current = ref(-1)

const step = computed(() => ONBOARDING_STEPS[Math.max(0, current.value)])
const isLast = computed(() => current.value === ONBOARDING_STEPS.length - 1)
const done = (i: number) => !!state.value?.[ONBOARDING_STEPS[i].key]
const canNext = computed(() => done(current.value))
const doneCount = computed(() => ONBOARDING_STEPS.filter((_, i) => done(i)).length)

async function refresh() {
  state.value = await onboardingApi.state()
}

onMounted(async () => {
  await refresh()
  const q = stepIndex(route.query.step)
  current.value = q >= 0 ? q : firstPendingStep(state.value!)
})

// 步骤写入地址栏，刷新或从清单进入时可直接定位
watch(current, (i) => {
  if (i < 0) return
  const key = ONBOARDING_STEPS[i].key
  if (route.query.step !== key) router.replace({ query: { step: key } })
})

// 已在本页时通过链接切换步骤（如 ⌘K、清单）
watch(
  () => route.query.step,
  (v) => {
    const i = stepIndex(v)
    if (i >= 0 && i !== current.value) current.value = i
  },
)

function go(i: number) {
  current.value = Math.min(Math.max(i, 0), ONBOARDING_STEPS.length - 1)
}

async function onChanged() {
  const before = done(current.value)
  await refresh()
  // 必做步骤刚完成时自动前进（运行步骤由用户查看结果后自行离开）
  if (!before && done(current.value) && current.value < 3 && !step.value.optional) go(current.value + 1)
}

function finish() {
  router.push('/')
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-3xl flex-col gap-5">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="page-title">快速开始</h1>
        <p class="text-muted-foreground mt-0.5">5 步完成配置，已完成 {{ doneCount }}/{{ ONBOARDING_STEPS.length }}</p>
      </div>
      <Button variant="ghost" size="sm" @click="finish">稍后再说</Button>
    </div>

    <!-- 步骤条 -->
    <ol class="grid grid-cols-5 gap-1.5 sm:gap-2">
      <li v-for="(s, i) in ONBOARDING_STEPS" :key="s.key">
        <button type="button" class="group flex w-full flex-col gap-1.5 text-left" @click="go(i)">
          <span
            :class="
              cn(
                'h-1 rounded-full transition-colors',
                i === current ? 'bg-primary' : done(i) ? 'bg-primary/40' : 'bg-muted',
              )
            "
          />
          <span
            :class="
              cn(
                'hidden items-center gap-1 text-xs sm:flex',
                i === current ? 'text-foreground font-medium' : 'text-muted-foreground group-hover:text-foreground',
              )
            "
          >
            <Check v-if="done(i)" class="text-success size-3.5" />{{ s.title }}
          </span>
        </button>
      </li>
    </ol>

    <Skeleton v-if="current < 0" class="h-72 rounded-xl" />
    <Card v-else>
      <CardHeader>
        <div class="text-muted-foreground text-xs">第 {{ current + 1 }} 步，共 {{ ONBOARDING_STEPS.length }} 步{{ step.optional ? ' · 可选' : '' }}</div>
        <CardTitle class="flex items-center gap-2 text-base">
          <component :is="step.icon" class="text-muted-foreground size-4" />{{ step.title }}
        </CardTitle>
        <CardDescription>{{ step.description }}</CardDescription>
      </CardHeader>
      <CardContent class="min-w-0">
        <StepCfst v-if="step.key === 'cfst'" @changed="onChanged" />
        <StepAccount v-else-if="step.key === 'account'" @changed="onChanged" />
        <StepNotifier v-else-if="step.key === 'notifier'" @changed="onChanged" />
        <StepTask v-else-if="step.key === 'task'" @changed="onChanged" />
        <StepRun v-else @changed="refresh" />
      </CardContent>
      <CardFooter class="flex items-center justify-between gap-2 border-t">
        <Button v-if="current > 0" variant="ghost" @click="go(current - 1)"><ArrowLeft />上一步</Button>
        <span v-else />
        <div class="flex gap-2">
          <Button v-if="current > 0 && !done(current) && !isLast" variant="outline" @click="go(current + 1)">跳过</Button>
          <Button v-if="isLast" :variant="state?.run ? 'default' : 'outline'" @click="finish"><LayoutDashboard />进入仪表盘</Button>
          <Button v-else :disabled="!canNext" @click="go(current + 1)">下一步<ArrowRight /></Button>
        </div>
      </CardFooter>
    </Card>
  </div>
</template>
