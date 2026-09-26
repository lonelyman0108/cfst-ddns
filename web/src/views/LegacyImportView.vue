<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { ArrowLeft, Bell, CircleCheck, FileUp, ListChecks, Loader2, ScanText, TriangleAlert, Upload, UserRoundKey } from '@lucide/vue'
import { legacyApi } from '@/api'
import type { LegacyApplyResult, LegacyPreview } from '@/api/types-p9'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import PageHeader from '@/components/PageHeader.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import BrandIcon from '@/components/BrandIcon.vue'
import { confirm } from '@/composables/useConfirm'
import { useMetaStore } from '@/stores/meta'
import type { IPType } from '@/api/types'
import { describeCron } from '@/utils/cron'
import { ipTypeLabel } from '@/utils/format'

const router = useRouter()
const meta = useMetaStore()

const content = ref('')
const dragging = ref(false)
const parsing = ref(false)
const applying = ref(false)
const preview = ref<LegacyPreview | null>(null)
const result = ref<LegacyApplyResult | null>(null)
const fileInput = ref<HTMLInputElement>()

const PLACEHOLDER = `# 粘贴 v1 的 config.sh、.env 或 docker-compose 的 environment 片段，例如：
DNS_PROVIDER=cloudflare
CF_API_TOKEN=xxxxxxxx
DNS_RECORD_NAMES="cdn.example.com"
CRON_SCHEDULE="0 */6 * * *"
CFST_PARAMS="-n 200 -t 4 -tl 200"`

const total = computed(() => {
  const p = preview.value
  return p ? p.accounts.length + p.notifiers.length + p.tasks.length : 0
})

onMounted(() => {
  meta.loadProviders().catch(() => {})
  meta.loadNotifiers().catch(() => {})
})

async function readFile(f: File | undefined | null) {
  if (!f) return
  if (f.size > 1024 * 1024) {
    toast.error('文件过大，请只粘贴相关配置')
    return
  }
  content.value = await f.text()
  preview.value = null
}

function onPick(e: Event) {
  const input = e.target as HTMLInputElement
  readFile(input.files?.[0])
  input.value = ''
}

function onDrop(e: DragEvent) {
  dragging.value = false
  readFile(e.dataTransfer?.files?.[0])
}

async function parse() {
  if (!content.value.trim()) {
    toast.error('请先粘贴或选择配置文件')
    return
  }
  parsing.value = true
  result.value = null
  try {
    preview.value = await legacyApi.preview(content.value)
  } catch {
    /* 已提示 */
  } finally {
    parsing.value = false
  }
}

async function apply() {
  const p = preview.value
  if (!p) return
  const ok = await confirm({
    title: '确认导入？',
    description: `将新建 ${p.accounts.length} 个 DNS 账号、${p.notifiers.length} 个通知渠道、${p.tasks.length} 个任务，不会修改已有配置。`,
    confirmText: '导入',
  })
  if (!ok) return
  applying.value = true
  try {
    result.value = await legacyApi.apply(content.value)
    toast.success('v1 配置已导入')
  } catch {
    /* 已提示 */
  } finally {
    applying.value = false
  }
}

function reset() {
  preview.value = null
  result.value = null
}

// 按字段 Schema 显示中文标签与选项名，未知字段原样显示
function configLines(kind: 'provider' | 'notifier', type: string, c: Record<string, string>) {
  const t = (kind === 'provider' ? meta.providers : meta.notifiers).find((m) => m.type === type)
  return Object.entries(c ?? {})
    .filter(([, v]) => v !== '')
    .map(([k, v]) => {
      const f = t?.fields.find((x) => x.key === k)
      return [f?.label ?? k, f?.options?.find((o) => o.value === v)?.label ?? v] as const
    })
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-4xl flex-col gap-4">
    <PageHeader title="从 v1 导入" description="把 v1（Bash 脚本版）的环境变量转换为账号、通知渠道和任务">
      <Button variant="outline" size="sm" @click="router.push('/settings#backup')"><ArrowLeft />返回设置</Button>
    </PageHeader>

    <!-- 结果 -->
    <Card v-if="result">
      <CardHeader>
        <CardTitle class="flex items-center gap-2"><CircleCheck class="text-success size-4" />导入完成</CardTitle>
        <CardDescription>凭据已加密保存，建议在任务页检查一次目标记录后再执行</CardDescription>
      </CardHeader>
      <CardContent class="grid gap-4">
        <div class="grid grid-cols-3 gap-3">
          <div class="rounded-lg border p-3">
            <div class="text-muted-foreground text-xs">DNS 账号</div>
            <div class="stat-number mt-1">{{ result.created.accounts }}</div>
          </div>
          <div class="rounded-lg border p-3">
            <div class="text-muted-foreground text-xs">通知渠道</div>
            <div class="stat-number mt-1">{{ result.created.notifiers }}</div>
          </div>
          <div class="rounded-lg border p-3">
            <div class="text-muted-foreground text-xs">任务</div>
            <div class="stat-number mt-1">{{ result.created.tasks }}</div>
          </div>
        </div>
        <Alert v-if="result.warnings?.length" class="border-warning/40 bg-warning/5">
          <TriangleAlert class="text-warning!" />
          <AlertTitle>需要手动处理</AlertTitle>
          <AlertDescription>
            <ul class="list-disc pl-4">
              <li v-for="(w, i) in result.warnings" :key="i">{{ w }}</li>
            </ul>
          </AlertDescription>
        </Alert>
      </CardContent>
      <CardFooter class="flex flex-wrap justify-end gap-2 border-t">
        <Button variant="outline" @click="router.push('/accounts')">查看账号</Button>
        <Button @click="router.push('/tasks')"><ListChecks />查看任务</Button>
      </CardFooter>
    </Card>

    <!-- 预览 -->
    <template v-else-if="preview">
      <Alert v-if="preview.warnings?.length" class="border-warning/40 bg-warning/5">
        <TriangleAlert class="text-warning!" />
        <AlertTitle>{{ preview.warnings.length }} 项需要注意</AlertTitle>
        <AlertDescription>
          <ul class="list-disc pl-4">
            <li v-for="(w, i) in preview.warnings" :key="i">{{ w }}</li>
          </ul>
        </AlertDescription>
      </Alert>

      <Card v-if="!total" class="py-0">
        <CardContent class="text-muted-foreground py-10 text-center text-sm">没有识别到可导入的配置，请检查粘贴的内容</CardContent>
      </Card>

      <section v-if="preview.accounts.length" class="grid gap-2">
        <h2 class="flex items-center gap-2 text-sm font-medium"><UserRoundKey class="text-muted-foreground size-4" />DNS 账号 · {{ preview.accounts.length }}</h2>
        <div class="grid gap-3 sm:grid-cols-2">
          <Card v-for="(a, i) in preview.accounts" :key="i" class="gap-2 py-4">
            <CardHeader class="grid-cols-[auto_1fr] gap-x-3">
              <BrandIcon kind="provider" :type="a.provider" class="row-span-2 size-9" />
              <CardTitle class="truncate">{{ a.name }}</CardTitle>
              <CardDescription><ToneBadge tone="primary">{{ meta.providerName(a.provider) }}</ToneBadge></CardDescription>
            </CardHeader>
            <CardContent>
              <dl class="grid gap-1 text-xs">
                <div v-for="[k, v] in configLines('provider', a.provider, a.config)" :key="k" class="flex gap-2">
                  <dt class="text-muted-foreground w-28 shrink-0 truncate">{{ k }}</dt>
                  <dd class="min-w-0 truncate font-mono">{{ v }}</dd>
                </div>
              </dl>
            </CardContent>
          </Card>
        </div>
      </section>

      <section v-if="preview.notifiers.length" class="grid gap-2">
        <h2 class="flex items-center gap-2 text-sm font-medium"><Bell class="text-muted-foreground size-4" />通知渠道 · {{ preview.notifiers.length }}</h2>
        <div class="grid gap-3 sm:grid-cols-2">
          <Card v-for="(n, i) in preview.notifiers" :key="i" class="gap-2 py-4">
            <CardHeader class="grid-cols-[auto_1fr] gap-x-3">
              <BrandIcon kind="notifier" :type="n.type" class="row-span-2 size-9" />
              <CardTitle class="truncate">{{ n.name }}</CardTitle>
              <CardDescription class="flex flex-wrap gap-1.5">
                <ToneBadge tone="primary">{{ meta.notifierName(n.type) }}</ToneBadge>
                <ToneBadge v-if="n.onSuccess">成功时</ToneBadge>
                <ToneBadge v-if="n.onFailure">失败时</ToneBadge>
                <ToneBadge v-if="n.onChangeOnly">仅 IP 变化</ToneBadge>
              </CardDescription>
            </CardHeader>
            <CardContent>
              <dl class="grid gap-1 text-xs">
                <div v-for="[k, v] in configLines('notifier', n.type, n.config)" :key="k" class="flex gap-2">
                  <dt class="text-muted-foreground w-28 shrink-0 truncate">{{ k }}</dt>
                  <dd class="min-w-0 truncate font-mono">{{ v }}</dd>
                </div>
              </dl>
            </CardContent>
          </Card>
        </div>
      </section>

      <section v-if="preview.tasks.length" class="grid gap-2">
        <h2 class="flex items-center gap-2 text-sm font-medium"><ListChecks class="text-muted-foreground size-4" />任务 · {{ preview.tasks.length }}</h2>
        <Card v-for="(t, i) in preview.tasks" :key="i" class="gap-2 py-4">
          <CardHeader>
            <CardTitle class="truncate">{{ t.name }}</CardTitle>
            <CardDescription class="flex flex-wrap gap-1.5">
              <ToneBadge>{{ t.cron ? describeCron(t.cron) : '仅手动' }}</ToneBadge>
              <ToneBadge>{{ ipTypeLabel[t.ipType as IPType] ?? t.ipType }}</ToneBadge>
            </CardDescription>
          </CardHeader>
          <CardContent class="text-muted-foreground text-sm whitespace-pre-line">{{ t.summary }}</CardContent>
        </Card>
      </section>

      <p v-if="preview.settings?.githubMirror" class="text-muted-foreground text-sm">
        GitHub 镜像：<span class="text-foreground font-mono">{{ preview.settings.githubMirror }}</span>
      </p>

      <div class="flex flex-wrap justify-end gap-2">
        <Button variant="outline" @click="reset">返回修改</Button>
        <Button :disabled="!total || applying" @click="apply"><Loader2 v-if="applying" class="animate-spin" /><Upload v-else />确认导入</Button>
      </div>
    </template>

    <!-- 输入 -->
    <Card v-else>
      <CardHeader>
        <CardTitle class="flex items-center gap-2"><ScanText class="text-muted-foreground size-4" />粘贴旧配置</CardTitle>
        <CardDescription>支持 KEY=VALUE、export KEY=…、- KEY=VALUE 与 KEY: VALUE 写法；先预览，确认后才会创建</CardDescription>
      </CardHeader>
      <CardContent class="grid gap-3">
        <div
          :class="['relative rounded-md transition-shadow', dragging ? 'ring-primary ring-2' : '']"
          @dragover.prevent="dragging = true"
          @dragleave="dragging = false"
          @drop.prevent="onDrop"
        >
          <Textarea v-model="content" :placeholder="PLACEHOLDER" class="min-h-64 font-mono text-code" spellcheck="false" />
        </div>
        <p class="text-muted-foreground text-xs">也可以把 config.sh / .env / docker-compose.yml 拖到输入框中。内容只发送到本机服务。</p>
      </CardContent>
      <CardFooter class="flex flex-wrap justify-between gap-2 border-t">
        <Button variant="outline" @click="fileInput?.click()"><FileUp />选择文件</Button>
        <input ref="fileInput" type="file" accept=".sh,.env,.yml,.yaml,.txt,text/plain" class="hidden" @change="onPick" />
        <Button :disabled="parsing" @click="parse"><Loader2 v-if="parsing" class="animate-spin" /><ScanText v-else />解析预览</Button>
      </CardFooter>
    </Card>
  </div>
</template>
