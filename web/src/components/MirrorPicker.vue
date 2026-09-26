<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Check, Gauge, Loader2 } from '@lucide/vue'
import { cfstApi, settingsApi } from '@/api'
import type { MirrorPreset, MirrorProbe } from '@/api'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import ToneBadge from '@/components/ToneBadge.vue'
import type { Tone } from '@/utils/format'

// GitHub 镜像测速与选用：测速结果按可用性、延迟排序，选用后直接保存到系统设置
const props = withDefaults(defineProps<{ autoTest?: boolean }>(), { autoTest: true })
const emit = defineEmits<{ (e: 'selected', mirror: string): void }>()

const presets = ref<MirrorPreset[]>([])
const probes = ref<MirrorProbe[] | null>(null)
const current = ref<string | null>(null)
const loading = ref(true)
const testing = ref(false)
const saving = ref<string | null>(null)

interface Row {
  mirror: string
  label: string
  probe?: MirrorProbe
}

const rows = computed<Row[]>(() => {
  if (probes.value) return probes.value.map((p) => ({ mirror: p.mirror, label: p.label || p.mirror, probe: p }))
  const list: Row[] = presets.value.map((m) => ({ ...m }))
  if (current.value && !list.some((r) => r.mirror === current.value)) list.push({ mirror: current.value, label: '当前设置' })
  return list
})

const best = computed(() => probes.value?.find((p) => p.ok)?.mirror)

function latencyTone(p: MirrorProbe): Tone {
  if (!p.ok) return 'danger'
  if (p.latencyMs < 800) return 'success'
  if (p.latencyMs < 2000) return 'warning'
  return 'danger'
}

async function test() {
  testing.value = true
  try {
    probes.value = (await cfstApi.testMirrors()) ?? []
  } catch {
    /* 已提示 */
  } finally {
    testing.value = false
  }
}

async function use(mirror: string) {
  saving.value = mirror
  try {
    const s = await settingsApi.update({ githubMirror: mirror })
    current.value = s.githubMirror ?? mirror
    toast.success(mirror ? '已切换 GitHub 镜像' : '已切换为直连 GitHub')
    emit('selected', mirror)
  } catch {
    /* 已提示 */
  } finally {
    saving.value = null
  }
}

onMounted(async () => {
  try {
    const [m, s] = await Promise.all([cfstApi.mirrors(), settingsApi.get()])
    presets.value = m ?? []
    current.value = s.githubMirror ?? ''
  } catch {
    /* 已提示 */
  } finally {
    loading.value = false
  }
  if (props.autoTest) test()
})

defineExpose({ test })
</script>

<template>
  <div class="grid gap-3">
    <div class="flex items-center justify-between gap-2">
      <p class="text-muted-foreground text-xs">
        {{ testing ? '正在测速，最长约 8 秒…' : probes ? '已按可用性与延迟排序' : '测试各镜像下载 GitHub 资源的速度' }}
      </p>
      <Button variant="outline" size="sm" :disabled="testing || loading" @click="test">
        <Loader2 v-if="testing" class="animate-spin" /><Gauge v-else />{{ probes ? '重新测速' : '开始测速' }}
      </Button>
    </div>

    <div v-if="loading" class="grid gap-2"><Skeleton v-for="i in 4" :key="i" class="h-12" /></div>
    <div v-else-if="!rows.length" class="text-muted-foreground py-6 text-center text-sm">暂无可用镜像</div>
    <div v-else class="divide-y rounded-md border">
      <div v-for="r in rows" :key="r.mirror" class="flex items-center gap-3 px-3 py-2.5">
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-1.5 text-sm font-medium">
            <span class="truncate">{{ r.label }}</span>
            <ToneBadge v-if="best !== undefined && r.mirror === best" tone="success">最快</ToneBadge>
          </div>
          <div class="text-muted-foreground truncate font-mono text-xs" :title="r.mirror || 'https://github.com'">
            {{ r.mirror || 'https://github.com' }}
          </div>
          <div v-if="!testing && r.probe && !r.probe.ok && r.probe.error" class="text-destructive truncate text-xs" :title="r.probe.error">
            {{ r.probe.error }}
          </div>
        </div>
        <div class="w-20 shrink-0 text-right">
          <Skeleton v-if="testing" class="ml-auto h-5 w-14" />
          <ToneBadge v-else-if="r.probe" :tone="latencyTone(r.probe)">
            {{ r.probe.ok ? `${r.probe.latencyMs} ms` : '失败' }}
          </ToneBadge>
          <span v-else class="text-muted-foreground text-xs">—</span>
        </div>
        <div class="flex w-28 shrink-0 justify-end">
          <span v-if="current === r.mirror" class="text-success inline-flex h-8 items-center gap-1 text-xs font-medium"><Check class="size-3.5" />使用中</span>
          <Button v-else variant="outline" size="sm" :disabled="saving !== null" @click="use(r.mirror)">
            <Loader2 v-if="saving === r.mirror" class="animate-spin" />使用此镜像
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
