<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { useI18n } from 'vue-i18n'
import { CircleCheck, CloudDownload, FileUp, Loader2, RefreshCw, ScanSearch } from '@lucide/vue'
import { cfstApi } from '@/api'
import type { CfstStatus } from '@/api/types'
import type { CfstCandidate } from '@/api/types-p9'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import FormItem from '@/components/FormItem.vue'
import InlineLink from '@/components/InlineLink.vue'
import MirrorPicker from '@/components/MirrorPicker.vue'
import ToneBadge from '@/components/ToneBadge.vue'

const emit = defineEmits<{ (e: 'changed'): void }>()

const { t } = useI18n()
const status = ref<CfstStatus | null>(null)
const mode = ref<'online' | 'upload' | 'scan'>('online')
let poll: ReturnType<typeof setInterval> | undefined

async function loadStatus() {
  try {
    status.value = await cfstApi.status()
  } catch {
    /* 已提示 */
  }
  if (status.value?.installing && !poll) {
    poll = setInterval(async () => {
      const s = await cfstApi.status().catch(() => null)
      if (!s) return
      status.value = s
      if (!s.installing) {
        clearInterval(poll)
        poll = undefined
        if (s.installed) emit('changed')
      }
    }, 2000)
  }
}

onMounted(loadStatus)
onBeforeUnmount(() => clearInterval(poll))

// ---------- 在线安装（镜像选用由 MirrorPicker 直接保存到系统设置） ----------
const installing = ref(false)

async function install() {
  installing.value = true
  try {
    const r = await cfstApi.install('')
    toast.success(t('onboarding.cfst.installed', { v: r.version }))
    await loadStatus()
    emit('changed')
  } catch {
    /* 已提示 */
  } finally {
    installing.value = false
  }
}

// ---------- 上传本地包 ----------
const file = ref<File | null>(null)
const fileVersion = ref('')
const uploading = ref(false)
const uploadPct = ref(0)
const fileInput = ref<HTMLInputElement>()

function onPick(e: Event) {
  const input = e.target as HTMLInputElement
  file.value = input.files?.[0] ?? null
  input.value = ''
}

async function upload() {
  if (!file.value) return
  uploading.value = true
  uploadPct.value = 0
  try {
    const r = await cfstApi.upload(file.value, fileVersion.value.trim() || undefined, (p) => (uploadPct.value = p))
    toast.success(t('onboarding.cfst.imported', { v: r.version, os: r.os, arch: r.arch }))
    file.value = null
    await loadStatus()
    emit('changed')
  } catch {
    /* 已提示 */
  } finally {
    uploading.value = false
  }
}

// ---------- 自动识别 ----------
const candidates = ref<CfstCandidate[] | null>(null)
const scanning = ref(false)
const adopting = ref('')

async function scan() {
  scanning.value = true
  try {
    candidates.value = (await cfstApi.scan()) ?? []
  } catch {
    /* 已提示 */
  } finally {
    scanning.value = false
  }
}

async function adopt(c: CfstCandidate) {
  adopting.value = c.path
  try {
    const r = await cfstApi.adopt(c.path, c.version || undefined)
    toast.success(t('onboarding.cfst.adopted', { v: r.version }))
    await loadStatus()
    emit('changed')
  } catch {
    /* 已提示 */
  } finally {
    adopting.value = ''
  }
}
</script>

<template>
  <div class="grid grid-cols-1 gap-4">
    <Skeleton v-if="!status" class="h-24" />

    <div v-else-if="status.installed" class="border-success/40 bg-success/5 flex items-start gap-3 rounded-lg border p-4">
      <CircleCheck class="text-success mt-0.5 size-5 shrink-0" />
      <div class="min-w-0">
        <div class="font-medium">{{ t('onboarding.cfst.ready') }}</div>
        <p class="text-muted-foreground mt-0.5 text-sm">
          {{ t('onboarding.cfst.version', { v: status.version || t('onboarding.cfst.unknown') }) }} · {{ status.os }}/{{ status.arch }}
        </p>
        <p class="text-muted-foreground mt-0.5 font-mono text-code break-all">{{ status.path }}</p>
      </div>
    </div>

    <div v-else-if="status.installing" class="flex items-center gap-3 rounded-lg border p-4">
      <Loader2 class="text-primary size-5 animate-spin" />
      <div>
        <div class="font-medium">{{ t('onboarding.cfst.installing') }}</div>
        <p class="text-muted-foreground text-sm">{{ t('onboarding.cfst.installingHint') }}</p>
      </div>
    </div>

    <template v-else>
      <Alert class="border-warning/40 bg-warning/5">
        <CloudDownload class="text-warning!" />
        <AlertDescription>{{ t('onboarding.cfst.notInstalled') }}</AlertDescription>
      </Alert>

      <Tabs v-model="mode">
        <TabsList class="grid w-full grid-cols-3">
          <TabsTrigger value="online">{{ t('onboarding.cfst.tabs.online') }}</TabsTrigger>
          <TabsTrigger value="upload">{{ t('onboarding.cfst.tabs.upload') }}</TabsTrigger>
          <TabsTrigger value="scan">{{ t('onboarding.cfst.tabs.scan') }}</TabsTrigger>
        </TabsList>

        <TabsContent value="online" class="mt-3 grid grid-cols-1 gap-3">
          <p class="text-muted-foreground text-sm">{{ t('onboarding.cfst.onlineHint') }}</p>
          <MirrorPicker />
          <div>
            <Button :disabled="installing" @click="install">
              <Loader2 v-if="installing" class="animate-spin" /><CloudDownload v-else />{{ t('onboarding.cfst.installLatest') }}
            </Button>
          </div>
        </TabsContent>

        <TabsContent value="upload" class="mt-3 grid grid-cols-1 gap-3">
          <i18n-t keypath="onboarding.cfst.uploadHint" tag="p" class="text-muted-foreground text-sm">
            <template #link><InlineLink href="https://github.com/XIU2/CloudflareSpeedTest/releases">cfst Releases</InlineLink></template>
          </i18n-t>
          <button
            type="button"
            class="hover:border-primary/50 hover:bg-primary/5 flex flex-col items-center gap-2 rounded-lg border border-dashed px-4 py-6 text-center transition-colors"
            @click="fileInput?.click()"
          >
            <FileUp class="text-muted-foreground size-6" />
            <span class="text-sm font-medium">{{ file ? file.name : t('onboarding.cfst.pickFile') }}</span>
            <span class="text-muted-foreground text-xs">{{ file ? `${(file.size / 1024 / 1024).toFixed(1)} MB` : t('onboarding.cfst.maxSize') }}</span>
          </button>
          <input ref="fileInput" type="file" class="hidden" @change="onPick" />
          <FormItem :label="t('onboarding.cfst.versionLabel')" :help="t('onboarding.cfst.versionHelp')" for="cfst-ver">
            <Input id="cfst-ver" v-model="fileVersion" :placeholder="t('onboarding.cfst.versionPlaceholder')" class="max-w-56" />
          </FormItem>
          <div>
            <Button :disabled="!file || uploading" @click="upload">
              <Loader2 v-if="uploading" class="animate-spin" /><FileUp v-else />{{ uploading ? (uploadPct < 100 ? t('onboarding.cfst.uploadingPct', { n: uploadPct }) : t('onboarding.cfst.verifying')) : t('onboarding.cfst.uploadAndImport') }}
            </Button>
          </div>
        </TabsContent>

        <TabsContent value="scan" class="mt-3 grid grid-cols-1 gap-3">
          <div class="flex items-center justify-between gap-2">
            <p class="text-muted-foreground text-sm">{{ t('onboarding.cfst.scanHint') }}</p>
            <Button variant="outline" size="sm" :disabled="scanning" @click="scan">
              <Loader2 v-if="scanning" class="animate-spin" /><ScanSearch v-else />{{ candidates ? t('onboarding.cfst.rescan') : t('onboarding.cfst.startScan') }}
            </Button>
          </div>
          <p v-if="candidates && !candidates.length" class="text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm">
            {{ t('onboarding.cfst.noneFound') }}
          </p>
          <div v-for="c in candidates ?? []" :key="c.path" class="flex flex-wrap items-center gap-3 rounded-lg border p-3">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2 text-sm font-medium">
                {{ c.version || t('onboarding.cfst.versionUnknown') }}
                <ToneBadge>{{ t(`onboarding.cfst.sources.${c.source}`) }}</ToneBadge>
                <ToneBadge v-if="c.os" :tone="c.compatible ? 'success' : 'danger'">{{ c.os }}/{{ c.arch }}</ToneBadge>
              </div>
              <div class="text-muted-foreground mt-0.5 font-mono text-xs break-all">{{ c.path }}</div>
              <div v-if="!c.compatible && c.message" class="text-destructive mt-0.5 text-xs">{{ c.message }}</div>
            </div>
            <Button size="sm" :disabled="!c.compatible || !!adopting" @click="adopt(c)">
              <Loader2 v-if="adopting === c.path" class="animate-spin" />{{ t('onboarding.cfst.use') }}
            </Button>
          </div>
        </TabsContent>
      </Tabs>
    </template>

    <p v-if="status && !status.installed && !status.installing" class="text-xs">
      <InlineLink :icon="RefreshCw" @click="loadStatus">{{ t('onboarding.cfst.refreshStatus') }}</InlineLink>
    </p>
  </div>
</template>
