<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import {
  ArrowUpCircle,
  CircleAlert,
  CloudDownload,
  Download,
  ExternalLink,
  FileArchive,
  FileText,
  Gauge,
  Info,
  Loader2,
  PackageCheck,
  RefreshCw,
  RotateCcw,
  Save,
  ScanSearch,
  Upload,
  X,
} from '@lucide/vue'
import { cfstApi } from '@/api'
import type { CfstCandidate, CfstRelease, CfstStatus, IPFileKind } from '@/api'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import EmptyState from '@/components/EmptyState.vue'
import InlineLink from '@/components/InlineLink.vue'
import FormItem from '@/components/FormItem.vue'
import MirrorPicker from '@/components/MirrorPicker.vue'
import PageHeader from '@/components/PageHeader.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import { confirm } from '@/composables/useConfirm'
import { fmtTime, fromNow } from '@/utils/format'

const status = ref<CfstStatus | null>(null)
const releases = ref<CfstRelease[]>([])
const releasesLoading = ref(false)
const releasesError = ref(false)
const installing = ref('')

async function loadStatus() {
  try {
    status.value = await cfstApi.status()
  } catch {
    /* 已提示 */
  }
}

async function loadReleases() {
  releasesLoading.value = true
  releasesError.value = false
  try {
    releases.value = (await cfstApi.releases()) ?? []
  } catch {
    releasesError.value = true
  } finally {
    releasesLoading.value = false
  }
}

// 后端正在安装（如启动时自动安装）时轮询状态
let poll: ReturnType<typeof setInterval> | undefined
watch(
  () => !!status.value?.installing && !installing.value,
  (on) => {
    clearInterval(poll)
    if (on) poll = setInterval(loadStatus, 3000)
  },
)
onBeforeUnmount(() => clearInterval(poll))

const busy = computed(() => !!installing.value || !!status.value?.installing || upload.uploading || !!adopting.value)
const isCurrent = (tag: string) => !!status.value?.installed && status.value.version === tag
const releaseUrl = (tag: string) => `https://github.com/XIU2/CloudflareSpeedTest/releases/tag/${encodeURIComponent(tag)}`

/** 本平台可用的最新版本（按发布时间） */
const latest = computed(() =>
  releases.value
    .filter((r) => r.assetAvailable)
    .reduce<CfstRelease | undefined>((a, r) => (!a || r.publishedAt > a.publishedAt ? r : a), undefined),
)

function parseVersion(v: string): number[] | null {
  const m = /^v?(\d+)\.(\d+)(?:\.(\d+))?/.exec(v.trim())
  return m ? [Number(m[1]), Number(m[2]), Number(m[3] ?? 0)] : null
}

/** 已安装版本低于最新版时返回最新版；版本号无法识别时不提示 */
const upgrade = computed(() => {
  const s = status.value
  const l = latest.value
  if (!s?.installed || !l) return undefined
  const a = parseVersion(s.version)
  const b = parseVersion(l.tag)
  if (!a || !b) return undefined
  for (let i = 0; i < 3; i++) {
    if (a[i] !== b[i]) return a[i] < b[i] ? l : undefined
  }
  return undefined
})

async function install(version: string) {
  const label = version === 'latest' ? '最新版' : version
  if (status.value?.installed) {
    const ok = await confirm({
      title: `安装 ${label}？`,
      description: `将替换当前版本 ${status.value.version || ''}。下载与解压可能需要数分钟。`,
      confirmText: '安装',
    })
    if (!ok) return
  }
  installing.value = version
  const id = toast.loading(`正在安装 ${label}…`, { description: '下载与解压可能需要数分钟，请勿关闭页面' })
  try {
    const r = await cfstApi.install(version)
    toast.success(`已安装 ${r.version}`, { id, description: undefined })
    await loadStatus()
  } catch {
    toast.dismiss(id)
  } finally {
    installing.value = ''
  }
}

// ---------- 导入本地文件 ----------
const MAX_UPLOAD = 64 << 20
const upload = reactive({
  open: false,
  file: null as File | null,
  version: '',
  progress: 0,
  uploading: false,
  dragging: false,
  error: '',
})
const fileInput = ref<HTMLInputElement>()

function openUpload() {
  Object.assign(upload, { open: true, file: null, version: '', progress: 0, error: '', dragging: false })
}

function setFile(f: File | undefined | null) {
  upload.error = ''
  if (!f) return
  if (f.size > MAX_UPLOAD) {
    upload.error = '文件超过 64MB'
    return
  }
  upload.file = f
  // 文件名带版本号时预填，便于确认
  const m = /v\d+\.\d+(?:\.\d+)?/.exec(f.name)
  if (m && !upload.version) upload.version = m[0]
}

function onDrop(e: DragEvent) {
  upload.dragging = false
  setFile(e.dataTransfer?.files?.[0])
}

function onPick(e: Event) {
  const input = e.target as HTMLInputElement
  setFile(input.files?.[0])
  input.value = ''
}

const fmtSize = (n: number) => (n >= 1 << 20 ? `${(n / (1 << 20)).toFixed(1)} MB` : `${Math.max(1, Math.round(n / 1024))} KB`)

async function submitUpload() {
  if (!upload.file) return
  upload.uploading = true
  upload.progress = 0
  try {
    const r = await cfstApi.upload(upload.file, upload.version.trim() || undefined, (p) => (upload.progress = p))
    toast.success(`已导入 ${r.version}`, { description: `${r.os}/${r.arch}` })
    upload.open = false
    await loadStatus()
  } catch {
    /* 已提示 */
  } finally {
    upload.uploading = false
  }
}

// ---------- 自动识别 ----------
const scan = reactive({ open: false, loading: false, loaded: false, list: [] as CfstCandidate[] })
const adopting = ref('')
const sourceLabel: Record<CfstCandidate['source'], string> = { datadir: '数据目录', path: 'PATH', bundled: '镜像内置' }

async function runScan() {
  scan.open = true
  scan.loading = true
  try {
    scan.list = (await cfstApi.scan()) ?? []
    scan.loaded = true
  } catch {
    /* 已提示 */
  } finally {
    scan.loading = false
  }
}

async function adopt(c: CfstCandidate) {
  adopting.value = c.path
  try {
    const r = await cfstApi.adopt(c.path, c.version || undefined)
    toast.success(`已使用 ${r.version}`, { description: c.path })
    scan.open = false
    await loadStatus()
  } catch {
    /* 已提示 */
  } finally {
    adopting.value = ''
  }
}

// ---------- 镜像测速 ----------
const mirrorOpen = ref(false)

// ---------- IP 文件 ----------
const ipTab = ref<IPFileKind>('v4')
const ipFiles = reactive<Record<IPFileKind, { content: string; original: string; loading: boolean; saving: boolean; loaded: boolean }>>({
  v4: { content: '', original: '', loading: false, saving: false, loaded: false },
  v6: { content: '', original: '', loading: false, saving: false, loaded: false },
})

const countLines = (s: string) => s.split(/\r?\n/).filter((l) => l.trim() && !l.trim().startsWith('#')).length

async function loadIPFile(kind: IPFileKind) {
  const f = ipFiles[kind]
  f.loading = true
  try {
    const r = await cfstApi.getIPFile(kind)
    f.content = r.content ?? ''
    f.original = f.content
    f.loaded = true
  } catch {
    /* 已提示 */
  } finally {
    f.loading = false
  }
}

function onTab(v: unknown) {
  const k = (v === 'v6' ? 'v6' : 'v4') as IPFileKind
  ipTab.value = k
  if (!ipFiles[k].loaded) loadIPFile(k)
}

async function saveIPFile(kind: IPFileKind) {
  const f = ipFiles[kind]
  f.saving = true
  try {
    await cfstApi.saveIPFile(kind, f.content)
    f.original = f.content
    toast.success('IP 段文件已保存')
  } catch {
    /* 已提示 */
  } finally {
    f.saving = false
  }
}

async function resetIPFile(kind: IPFileKind) {
  const ok = await confirm({
    title: '恢复默认内容？',
    description: `${kind === 'v4' ? 'ip.txt' : 'ipv6.txt'} 将被恢复为 cfst 自带的默认 IP 段，当前内容会被覆盖。`,
    confirmText: '恢复默认',
    destructive: true,
  })
  if (!ok) return
  const f = ipFiles[kind]
  f.saving = true
  try {
    const r = await cfstApi.resetIPFile(kind)
    f.content = r.content ?? ''
    f.original = f.content
    toast.success('已恢复默认')
  } catch {
    /* 已提示 */
  } finally {
    f.saving = false
  }
}

onMounted(() => {
  loadStatus()
  loadReleases()
  loadIPFile('v4')
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="cfst 管理" description="CloudflareSpeedTest 可执行文件与 IP 段文件">
      <Button variant="outline" size="sm" @click="loadStatus(), loadReleases()"><RefreshCw />刷新</Button>
      <Button variant="outline" size="sm" :disabled="busy" @click="runScan"><ScanSearch />自动识别</Button>
      <Button variant="outline" size="sm" :disabled="busy" @click="openUpload"><Upload />导入本地文件</Button>
      <Button size="sm" :disabled="busy" @click="install('latest')">
        <Loader2 v-if="installing === 'latest'" class="animate-spin" /><CloudDownload v-else />安装最新版
      </Button>
    </PageHeader>

    <Alert>
      <Info />
      <AlertDescription>
        <p>
          cfst 从 GitHub Releases 下载（<a href="https://github.com/XIU2/CloudflareSpeedTest" target="_blank" rel="noopener" class="text-primary hover:underline">XIU2/CloudflareSpeedTest</a>）。
          下载缓慢时可
          <InlineLink :icon="Gauge" @click="mirrorOpen = true">测试镜像速度</InlineLink>
          并一键切换；无法联网时可导入本地文件。
        </p>
      </AlertDescription>
    </Alert>

    <Alert v-if="upgrade && !busy" class="border-primary/40 bg-primary/5">
      <ArrowUpCircle class="text-primary!" />
      <AlertTitle>有新版本 {{ upgrade.tag }}</AlertTitle>
      <AlertDescription class="flex flex-wrap items-center justify-between gap-2">
        <span>当前安装 {{ status?.version }}，发布于 {{ fromNow(upgrade.publishedAt) }}。</span>
        <Button size="sm" @click="install(upgrade.tag)"><Download />升级</Button>
      </AlertDescription>
    </Alert>

    <div class="grid gap-4 lg:grid-cols-3">
      <Card class="min-w-0 lg:col-span-1">
        <CardHeader>
          <CardTitle class="flex items-center gap-2"><PackageCheck class="text-muted-foreground size-4" />当前安装</CardTitle>
          <CardAction>
            <ToneBadge v-if="status" :tone="status.installed ? 'success' : 'danger'">{{ status.installed ? '已安装' : '未安装' }}</ToneBadge>
          </CardAction>
        </CardHeader>
        <CardContent class="grid gap-4">
          <div v-if="!status" class="grid gap-3"><Skeleton v-for="i in 4" :key="i" class="h-8" /></div>
          <template v-else>
            <div>
              <div class="stat-number">{{ status.version || '—' }}</div>
              <div class="text-muted-foreground mt-1 text-xs">{{ status.os }}/{{ status.arch }}</div>
            </div>
            <dl class="grid gap-3 text-sm">
              <div>
                <dt class="text-muted-foreground text-xs">路径</dt>
                <dd class="mt-0.5 font-mono text-code break-all">{{ status.path || '-' }}</dd>
              </div>
              <div>
                <dt class="text-muted-foreground text-xs">本平台资源文件</dt>
                <dd class="mt-0.5 font-mono text-code">{{ status.asset || '-' }}</dd>
              </div>
            </dl>
            <div v-if="!status.installed && !busy" class="grid gap-2">
              <p class="text-muted-foreground text-xs">可在线安装、导入本地文件，或识别本机已有的 cfst。</p>
              <div class="flex flex-wrap gap-2">
                <Button size="sm" @click="install('latest')"><CloudDownload />安装最新版</Button>
                <Button variant="outline" size="sm" @click="openUpload"><Upload />导入</Button>
                <Button variant="outline" size="sm" @click="runScan"><ScanSearch />识别</Button>
              </div>
            </div>
            <Alert v-if="installing || status.installing" class="border-info/40 bg-info/5">
              <Loader2 class="text-info! animate-spin" />
              <AlertTitle class="text-info">正在安装{{ installing && installing !== 'latest' ? ` ${installing}` : '' }}</AlertTitle>
              <AlertDescription>下载与解压可能需要数分钟，请勿关闭页面。</AlertDescription>
            </Alert>
          </template>
        </CardContent>
      </Card>

      <Card class="min-w-0 gap-0 py-0 lg:col-span-2">
        <CardHeader class="border-b py-3 [.border-b]:pb-3">
          <CardTitle class="flex items-center gap-2"><CloudDownload class="text-muted-foreground size-4" />可用版本</CardTitle>
          <CardDescription>GitHub Releases（Releases 列表直连 GitHub API）</CardDescription>
        </CardHeader>
        <CardContent class="p-0">
          <div v-if="releasesLoading && !releases.length" class="grid gap-3 p-5"><Skeleton v-for="i in 5" :key="i" class="h-8" /></div>
          <EmptyState v-else-if="releasesError" :icon="CircleAlert" compact title="获取版本列表失败" description="可能无法访问 GitHub API，请检查网络后重试">
            <Button variant="outline" size="sm" @click="loadReleases"><RefreshCw />重试</Button>
          </EmptyState>
          <div v-else class="max-h-[420px] overflow-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead class="pl-5">版本</TableHead>
                  <TableHead>发布时间</TableHead>
                  <TableHead class="w-44 pr-5 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="r in releases" :key="r.tag">
                  <TableCell class="pl-5">
                    <div class="flex items-center gap-2">
                      <span class="font-mono text-sm font-medium">{{ r.tag }}</span>
                      <ToneBadge v-if="isCurrent(r.tag)" tone="success">当前</ToneBadge>
                      <ToneBadge v-if="latest?.tag === r.tag" tone="primary">最新</ToneBadge>
                    </div>
                    <div v-if="r.name && r.name !== r.tag" class="text-muted-foreground max-w-60 truncate text-xs">{{ r.name }}</div>
                  </TableCell>
                  <TableCell class="text-xs whitespace-nowrap" :title="fmtTime(r.publishedAt)">
                    {{ fmtTime(r.publishedAt, 'YYYY-MM-DD') }} <span class="text-muted-foreground">· {{ fromNow(r.publishedAt) }}</span>
                  </TableCell>
                  <TableCell class="w-44 pr-5">
                    <div class="flex items-center justify-end gap-1">
                      <Button v-if="r.assetAvailable" size="sm" variant="outline" class="w-24" :disabled="busy" @click="install(r.tag)">
                        <Loader2 v-if="installing === r.tag" class="animate-spin" /><RotateCcw v-else-if="isCurrent(r.tag)" /><Download v-else />
                        {{ isCurrent(r.tag) ? '重新安装' : '安装' }}
                      </Button>
                      <Tooltip v-else>
                        <TooltipTrigger as-child>
                          <ToneBadge tone="neutral" class="w-24 cursor-default justify-center" tabindex="0">不支持本平台</ToneBadge>
                        </TooltipTrigger>
                        <TooltipContent>该版本没有 {{ status ? `${status.os}/${status.arch}` : '本平台' }} 的资源文件</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger as-child>
                          <Button variant="ghost" size="icon" class="text-muted-foreground size-8" as-child>
                            <a :href="releaseUrl(r.tag)" target="_blank" rel="noopener" :aria-label="`在 GitHub 查看 ${r.tag}`"><ExternalLink /></a>
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>在 GitHub 查看</TooltipContent>
                      </Tooltip>
                    </div>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>
    </div>

    <Card>
      <CardHeader>
        <CardTitle class="flex items-center gap-2"><FileText class="text-muted-foreground size-4" />IP 段文件</CardTitle>
        <CardDescription>任务「IP 来源」为默认时使用，每行一个 IP 或 CIDR 段，# 开头为注释</CardDescription>
      </CardHeader>
      <CardContent>
        <Tabs :model-value="ipTab" @update:model-value="onTab">
          <TabsList>
            <TabsTrigger value="v4">IPv4 · ip.txt</TabsTrigger>
            <TabsTrigger value="v6">IPv6 · ipv6.txt</TabsTrigger>
          </TabsList>
          <TabsContent v-for="k in ['v4', 'v6'] as const" :key="k" :value="k" class="mt-3">
            <Skeleton v-if="ipFiles[k].loading && !ipFiles[k].loaded" class="h-72" />
            <template v-else>
              <Textarea
                v-model="ipFiles[k].content"
                spellcheck="false"
                class="scrollbar-thin h-80 resize-y font-mono text-log"
                placeholder="每行一个 IP 或 CIDR 段"
              />
              <div class="mt-3 flex flex-wrap items-center gap-2">
                <span class="text-muted-foreground text-xs">
                  {{ countLines(ipFiles[k].content) }} 行有效内容
                  <span v-if="ipFiles[k].content !== ipFiles[k].original" class="text-warning"> · 未保存</span>
                </span>
                <div class="ml-auto flex gap-2">
                  <Button variant="outline" size="sm" :disabled="ipFiles[k].saving" @click="resetIPFile(k)"><RotateCcw />恢复默认</Button>
                  <Button size="sm" :disabled="ipFiles[k].saving || ipFiles[k].content === ipFiles[k].original" @click="saveIPFile(k)">
                    <Loader2 v-if="ipFiles[k].saving" class="animate-spin" /><Save v-else />保存
                  </Button>
                </div>
              </div>
            </template>
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>

    <!-- 导入本地文件 -->
    <Dialog v-model:open="upload.open">
      <DialogContent class="sm:max-w-lg" @interact-outside="upload.uploading && $event.preventDefault()">
        <DialogHeader>
          <DialogTitle>导入本地文件</DialogTitle>
          <DialogDescription>支持 release 压缩包（.zip / .tar.gz）或解压后的可执行文件，最大 64MB</DialogDescription>
        </DialogHeader>
        <div class="grid gap-4">
          <div
            role="button"
            tabindex="0"
            :class="[
              'flex cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border border-dashed px-4 py-8 text-center transition-colors',
              upload.dragging ? 'border-primary bg-primary/5' : 'hover:border-primary/50 hover:bg-muted/40',
              upload.uploading ? 'pointer-events-none opacity-60' : '',
            ]"
            @click="fileInput?.click()"
            @keydown.enter.prevent="fileInput?.click()"
            @keydown.space.prevent="fileInput?.click()"
            @dragenter.prevent="upload.dragging = true"
            @dragover.prevent="upload.dragging = true"
            @dragleave.prevent="upload.dragging = false"
            @drop.prevent="onDrop"
          >
            <Upload class="text-muted-foreground size-6" />
            <div class="text-sm">拖入文件，或 <span class="text-primary">点击选择</span></div>
            <div class="text-muted-foreground text-xs">会校验文件的系统与架构是否与本机（{{ status ? `${status.os}/${status.arch}` : '—' }}）一致</div>
            <input ref="fileInput" type="file" class="hidden" @change="onPick" />
          </div>
          <p v-if="upload.error" class="text-destructive text-xs">{{ upload.error }}</p>

          <div v-if="upload.file" class="flex items-center gap-3 rounded-md border px-3 py-2">
            <FileArchive class="text-muted-foreground size-5 shrink-0" />
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-medium">{{ upload.file.name }}</div>
              <div class="text-muted-foreground text-xs">{{ fmtSize(upload.file.size) }}</div>
            </div>
            <Button v-if="!upload.uploading" variant="ghost" size="icon" class="size-7" aria-label="移除" @click="upload.file = null"><X /></Button>
          </div>

          <FormItem label="版本号" for="cfst-version" help="留空时从文件名或程序输出中识别">
            <Input id="cfst-version" v-model="upload.version" placeholder="如 v2.3.4" class="font-mono" :disabled="upload.uploading" />
          </FormItem>

          <div v-if="upload.uploading" class="grid gap-1.5">
            <Progress :model-value="upload.progress" />
            <div class="text-muted-foreground text-xs">{{ upload.progress < 100 ? `上传中 ${upload.progress}%` : '正在校验与安装…' }}</div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" :disabled="upload.uploading" @click="upload.open = false">取消</Button>
          <Button :disabled="!upload.file || upload.uploading" @click="submitUpload">
            <Loader2 v-if="upload.uploading" class="animate-spin" /><Upload v-else />导入
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- 自动识别 -->
    <Dialog v-model:open="scan.open">
      <DialogContent class="flex max-h-[85vh] flex-col sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>自动识别</DialogTitle>
          <DialogDescription>在数据目录、PATH 和镜像内置目录中查找已有的 cfst</DialogDescription>
        </DialogHeader>
        <div class="-mx-6 min-h-0 flex-1 overflow-y-auto px-6">
          <div v-if="scan.loading" class="grid gap-2"><Skeleton v-for="i in 3" :key="i" class="h-16" /></div>
          <EmptyState
            v-else-if="!scan.list.length"
            :icon="ScanSearch"
            compact
            :title="scan.loaded ? '未找到可用的 cfst' : '识别失败'"
            description="可以在线安装，或导入本地文件"
          >
            <Button variant="outline" size="sm" @click="(scan.open = false), openUpload()"><Upload />导入本地文件</Button>
          </EmptyState>
          <div v-else class="divide-y rounded-md border">
            <div v-for="c in scan.list" :key="c.path" class="flex items-center gap-3 px-3 py-2.5">
              <div class="min-w-0 flex-1">
                <div class="font-mono text-code break-all">{{ c.path }}</div>
                <div class="mt-1 flex flex-wrap items-center gap-1.5">
                  <ToneBadge>{{ sourceLabel[c.source] ?? c.source }}</ToneBadge>
                  <ToneBadge>{{ c.version || '版本未知' }}</ToneBadge>
                  <ToneBadge v-if="c.os || c.arch">{{ c.os || '?' }}/{{ c.arch || '?' }}</ToneBadge>
                  <ToneBadge :tone="c.compatible ? 'success' : 'danger'">{{ c.compatible ? '可用' : '不兼容' }}</ToneBadge>
                </div>
                <div v-if="!c.compatible && c.message" class="text-destructive mt-1 text-xs">{{ c.message }}</div>
              </div>
              <Button size="sm" variant="outline" class="shrink-0" :disabled="!c.compatible || !!adopting" @click="adopt(c)">
                <Loader2 v-if="adopting === c.path" class="animate-spin" />使用
              </Button>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" :disabled="scan.loading" @click="runScan"><RefreshCw />重新识别</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- 镜像测速 -->
    <Dialog v-model:open="mirrorOpen">
      <DialogContent class="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>测试镜像速度</DialogTitle>
          <DialogDescription>选用后用于下载 cfst，也可在系统设置中修改</DialogDescription>
        </DialogHeader>
        <MirrorPicker />
      </DialogContent>
    </Dialog>
  </div>
</template>
