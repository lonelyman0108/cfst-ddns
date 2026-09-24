<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { Check, CircleAlert, CloudDownload, FileText, Info, Loader2, PackageCheck, RefreshCw, RotateCcw, Save } from '@lucide/vue'
import { cfstApi } from '@/api'
import type { CfstRelease, CfstStatus, IPFileKind } from '@/api/types'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import EmptyState from '@/components/EmptyState.vue'
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

const busy = computed(() => !!installing.value || !!status.value?.installing)
const isCurrent = (tag: string) => !!status.value?.installed && status.value.version === tag

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
      <Button size="sm" :disabled="busy" @click="install('latest')">
        <Loader2 v-if="installing === 'latest'" class="animate-spin" /><CloudDownload v-else />安装最新版
      </Button>
    </PageHeader>

    <Alert>
      <Info />
      <AlertDescription>
        <p>
          cfst 从 GitHub Releases 下载（<a href="https://github.com/XIU2/CloudflareSpeedTest" target="_blank" rel="noopener" class="text-primary hover:underline">XIU2/CloudflareSpeedTest</a>）。
          国内网络下载缓慢时，可在 <router-link to="/settings" class="text-primary hover:underline">系统设置</router-link> 中配置 GitHub 镜像。
        </p>
      </AlertDescription>
    </Alert>

    <div class="grid gap-4 lg:grid-cols-3">
      <Card class="lg:col-span-1">
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
            <Alert v-if="busy" class="border-info/40 bg-info/5">
              <Loader2 class="text-info! animate-spin" />
              <AlertTitle class="text-info">正在安装{{ installing && installing !== 'latest' ? ` ${installing}` : '' }}</AlertTitle>
              <AlertDescription>下载与解压可能需要数分钟，请勿关闭页面。</AlertDescription>
            </Alert>
          </template>
        </CardContent>
      </Card>

      <Card class="gap-0 py-0 lg:col-span-2">
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
                  <TableHead class="text-center">本平台资源</TableHead>
                  <TableHead class="pr-5 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="r in releases" :key="r.tag">
                  <TableCell class="pl-5">
                    <div class="flex items-center gap-2">
                      <span class="font-mono text-sm font-medium">{{ r.tag }}</span>
                      <ToneBadge v-if="isCurrent(r.tag)" tone="success">当前</ToneBadge>
                    </div>
                    <div v-if="r.name && r.name !== r.tag" class="text-muted-foreground max-w-60 truncate text-xs">{{ r.name }}</div>
                  </TableCell>
                  <TableCell class="text-xs" :title="fmtTime(r.publishedAt)">
                    {{ fmtTime(r.publishedAt, 'YYYY-MM-DD') }} <span class="text-muted-foreground">· {{ fromNow(r.publishedAt) }}</span>
                  </TableCell>
                  <TableCell class="text-center">
                    <Check v-if="r.assetAvailable" class="text-success mx-auto size-4" />
                    <span v-else class="text-muted-foreground text-xs">无</span>
                  </TableCell>
                  <TableCell class="pr-5 text-right">
                    <Button
                      size="sm"
                      :variant="isCurrent(r.tag) ? 'ghost' : 'outline'"
                      :disabled="!r.assetAvailable || busy"
                      @click="install(r.tag)"
                    >
                      <Loader2 v-if="installing === r.tag" class="animate-spin" />
                      {{ isCurrent(r.tag) ? '重新安装' : '安装此版本' }}
                    </Button>
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
  </div>
</template>
