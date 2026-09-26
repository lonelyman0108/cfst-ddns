<script setup lang="ts">
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ArrowLeft } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import RunLogViewer from '@/components/RunLogViewer.vue'

const route = useRoute()
const router = useRouter()
const runId = computed(() => Number(route.params.id))

function back() {
  if (window.history.state?.back) router.back()
  else router.push('/runs')
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-6xl flex-col gap-4">
    <div class="flex items-center gap-3">
      <Button variant="ghost" size="icon" class="size-8" @click="back"><ArrowLeft /></Button>
      <div>
        <h1 class="page-title">
          执行详情 <span class="text-muted-foreground font-mono">#{{ runId }}</span>
        </h1>
        <p class="text-muted-foreground text-sm">测速结果、DNS 变更与完整日志</p>
      </div>
    </div>
    <RunLogViewer :run-id="runId" log-height="520px" />
  </div>
</template>
