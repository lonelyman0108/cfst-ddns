<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
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
    toast.success(`cfst ${r.version} 安装完成`)
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
    toast.success(`已导入 cfst ${r.version}（${r.os}/${r.arch}）`)
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
const sourceLabel: Record<CfstCandidate['source'], string> = { datadir: '数据目录', path: 'PATH', bundled: '镜像内置' }

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
    toast.success(`已使用 cfst ${r.version}`)
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
  <div class="grid gap-4">
    <Skeleton v-if="!status" class="h-24" />

    <div v-else-if="status.installed" class="border-success/40 bg-success/5 flex items-start gap-3 rounded-lg border p-4">
      <CircleCheck class="text-success mt-0.5 size-5 shrink-0" />
      <div class="min-w-0">
        <div class="font-medium">cfst 已就绪</div>
        <p class="text-muted-foreground mt-0.5 text-sm">
          版本 {{ status.version || '未知' }} · {{ status.os }}/{{ status.arch }}
        </p>
        <p class="text-muted-foreground mt-0.5 font-mono text-code break-all">{{ status.path }}</p>
      </div>
    </div>

    <div v-else-if="status.installing" class="flex items-center gap-3 rounded-lg border p-4">
      <Loader2 class="text-primary size-5 animate-spin" />
      <div>
        <div class="font-medium">正在安装 cfst…</div>
        <p class="text-muted-foreground text-sm">通常需要几十秒，完成后自动进入下一步</p>
      </div>
    </div>

    <template v-else>
      <Alert class="border-warning/40 bg-warning/5">
        <CloudDownload class="text-warning!" />
        <AlertDescription>还没有安装 cfst。选择一种方式准备好它，任务才能测速。</AlertDescription>
      </Alert>

      <Tabs v-model="mode">
        <TabsList class="grid w-full grid-cols-3">
          <TabsTrigger value="online">在线安装</TabsTrigger>
          <TabsTrigger value="upload">上传本地包</TabsTrigger>
          <TabsTrigger value="scan">自动识别</TabsTrigger>
        </TabsList>

        <TabsContent value="online" class="mt-3 grid gap-3">
          <p class="text-muted-foreground text-sm">从 GitHub 下载最新版。国内网络建议先选用一个延迟低的镜像。</p>
          <MirrorPicker />
          <div>
            <Button :disabled="installing" @click="install">
              <Loader2 v-if="installing" class="animate-spin" /><CloudDownload v-else />安装最新版
            </Button>
          </div>
        </TabsContent>

        <TabsContent value="upload" class="mt-3 grid gap-3">
          <p class="text-muted-foreground text-sm">
            从
            <InlineLink href="https://github.com/XIU2/CloudflareSpeedTest/releases">cfst Releases</InlineLink>
            下载与本机平台一致的压缩包（zip / tar.gz）或解压后的程序，再上传到这里。
          </p>
          <button
            type="button"
            class="hover:border-primary/50 hover:bg-primary/5 flex flex-col items-center gap-2 rounded-lg border border-dashed px-4 py-6 text-center transition-colors"
            @click="fileInput?.click()"
          >
            <FileUp class="text-muted-foreground size-6" />
            <span class="text-sm font-medium">{{ file ? file.name : '选择文件' }}</span>
            <span class="text-muted-foreground text-xs">{{ file ? `${(file.size / 1024 / 1024).toFixed(1)} MB` : '最大 64 MB' }}</span>
          </button>
          <input ref="fileInput" type="file" class="hidden" @change="onPick" />
          <FormItem label="版本号" help="可选，留空则自动识别" for="cfst-ver">
            <Input id="cfst-ver" v-model="fileVersion" placeholder="如 v2.3.4" class="max-w-56" />
          </FormItem>
          <div>
            <Button :disabled="!file || uploading" @click="upload">
              <Loader2 v-if="uploading" class="animate-spin" /><FileUp v-else />{{ uploading ? (uploadPct < 100 ? `上传中 ${uploadPct}%` : '校验中…') : '上传并导入' }}
            </Button>
          </div>
        </TabsContent>

        <TabsContent value="scan" class="mt-3 grid gap-3">
          <div class="flex items-center justify-between gap-2">
            <p class="text-muted-foreground text-sm">查找数据目录、PATH 和镜像内置目录中已有的 cfst</p>
            <Button variant="outline" size="sm" :disabled="scanning" @click="scan">
              <Loader2 v-if="scanning" class="animate-spin" /><ScanSearch v-else />{{ candidates ? '重新识别' : '开始识别' }}
            </Button>
          </div>
          <p v-if="candidates && !candidates.length" class="text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm">
            没有找到可用的 cfst，请改用在线安装或上传
          </p>
          <div v-for="c in candidates ?? []" :key="c.path" class="flex flex-wrap items-center gap-3 rounded-lg border p-3">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2 text-sm font-medium">
                {{ c.version || '版本未知' }}
                <ToneBadge>{{ sourceLabel[c.source] ?? c.source }}</ToneBadge>
                <ToneBadge v-if="c.os" :tone="c.compatible ? 'success' : 'danger'">{{ c.os }}/{{ c.arch }}</ToneBadge>
              </div>
              <div class="text-muted-foreground mt-0.5 font-mono text-xs break-all">{{ c.path }}</div>
              <div v-if="!c.compatible && c.message" class="text-destructive mt-0.5 text-xs">{{ c.message }}</div>
            </div>
            <Button size="sm" :disabled="!c.compatible || !!adopting" @click="adopt(c)">
              <Loader2 v-if="adopting === c.path" class="animate-spin" />使用
            </Button>
          </div>
        </TabsContent>
      </Tabs>
    </template>

    <p v-if="status && !status.installed && !status.installing" class="text-xs">
      <InlineLink :icon="RefreshCw" @click="loadStatus">刷新状态</InlineLink>
    </p>
  </div>
</template>
