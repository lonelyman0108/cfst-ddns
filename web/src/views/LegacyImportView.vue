<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
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

const { t } = useI18n()
const router = useRouter()
const meta = useMetaStore()

const content = ref('')
const dragging = ref(false)
const parsing = ref(false)
const applying = ref(false)
const preview = ref<LegacyPreview | null>(null)
const result = ref<LegacyApplyResult | null>(null)
const fileInput = ref<HTMLInputElement>()

const placeholder = computed(
  () => `# ${t('legacy.placeholderComment')}
DNS_PROVIDER=cloudflare
CF_API_TOKEN=xxxxxxxx
DNS_RECORD_NAMES="cdn.example.com"
CRON_SCHEDULE="0 */6 * * *"
CFST_PARAMS="-n 200 -t 4 -tl 200"`,
)

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
    toast.error(t('legacy.fileTooLarge'))
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
    toast.error(t('legacy.contentRequired'))
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
    title: t('legacy.confirmTitle'),
    description: t('legacy.confirmDescription', { accounts: p.accounts.length, notifiers: p.notifiers.length, tasks: p.tasks.length }),
    confirmText: t('common.import'),
  })
  if (!ok) return
  applying.value = true
  try {
    result.value = await legacyApi.apply(content.value)
    toast.success(t('legacy.imported'))
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

// 按字段 Schema 显示本地化标签与选项名，未知字段原样显示
function configLines(kind: 'provider' | 'notifier', type: string, c: Record<string, string>) {
  const m = (kind === 'provider' ? meta.providers : meta.notifiers).find((m) => m.type === type)
  return Object.entries(c ?? {})
    .filter(([, v]) => v !== '')
    .map(([k, v]) => {
      const f = m?.fields.find((x) => x.key === k)
      return [f?.label ?? k, f?.options?.find((o) => o.value === v)?.label ?? v] as const
    })
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-4xl flex-col gap-4">
    <PageHeader :title="t('legacy.title')" :description="t('legacy.description')">
      <Button variant="outline" size="sm" @click="router.push('/settings#backup')"><ArrowLeft />{{ t('legacy.backToSettings') }}</Button>
    </PageHeader>

    <!-- 结果 -->
    <Card v-if="result">
      <CardHeader>
        <CardTitle class="flex items-center gap-2"><CircleCheck class="text-success size-4" />{{ t('legacy.doneTitle') }}</CardTitle>
        <CardDescription>{{ t('legacy.doneDescription') }}</CardDescription>
      </CardHeader>
      <CardContent class="grid grid-cols-1 gap-4">
        <div class="grid grid-cols-3 gap-3">
          <div class="rounded-lg border p-3">
            <div class="text-muted-foreground text-xs">{{ t('legacy.accounts') }}</div>
            <div class="stat-number mt-1">{{ result.created.accounts }}</div>
          </div>
          <div class="rounded-lg border p-3">
            <div class="text-muted-foreground text-xs">{{ t('legacy.notifiers') }}</div>
            <div class="stat-number mt-1">{{ result.created.notifiers }}</div>
          </div>
          <div class="rounded-lg border p-3">
            <div class="text-muted-foreground text-xs">{{ t('legacy.tasks') }}</div>
            <div class="stat-number mt-1">{{ result.created.tasks }}</div>
          </div>
        </div>
        <Alert v-if="result.warnings?.length" class="border-warning/40 bg-warning/5">
          <TriangleAlert class="text-warning!" />
          <AlertTitle>{{ t('legacy.manualAction') }}</AlertTitle>
          <AlertDescription>
            <ul class="list-disc pl-4">
              <li v-for="(w, i) in result.warnings" :key="i">{{ w }}</li>
            </ul>
          </AlertDescription>
        </Alert>
      </CardContent>
      <CardFooter class="flex flex-wrap justify-end gap-2 border-t">
        <Button variant="outline" @click="router.push('/accounts')">{{ t('legacy.viewAccounts') }}</Button>
        <Button @click="router.push('/tasks')"><ListChecks />{{ t('legacy.viewTasks') }}</Button>
      </CardFooter>
    </Card>

    <!-- 预览 -->
    <template v-else-if="preview">
      <Alert v-if="preview.warnings?.length" class="border-warning/40 bg-warning/5">
        <TriangleAlert class="text-warning!" />
        <AlertTitle>{{ t('legacy.warnings', { n: preview.warnings.length }) }}</AlertTitle>
        <AlertDescription>
          <ul class="list-disc pl-4">
            <li v-for="(w, i) in preview.warnings" :key="i">{{ w }}</li>
          </ul>
        </AlertDescription>
      </Alert>

      <Card v-if="!total" class="py-0">
        <CardContent class="text-muted-foreground py-10 text-center text-sm">{{ t('legacy.nothing') }}</CardContent>
      </Card>

      <section v-if="preview.accounts.length" class="grid grid-cols-1 gap-2">
        <h2 class="flex items-center gap-2 text-sm font-medium"><UserRoundKey class="text-muted-foreground size-4" />{{ t('legacy.accounts') }} · {{ preview.accounts.length }}</h2>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Card v-for="(a, i) in preview.accounts" :key="i" class="gap-2 py-4">
            <CardHeader class="grid-cols-[auto_1fr] gap-x-3">
              <BrandIcon kind="provider" :type="a.provider" class="row-span-2 size-9" />
              <CardTitle class="truncate" :title="a.name">{{ a.name }}</CardTitle>
              <CardDescription><ToneBadge tone="primary">{{ meta.providerName(a.provider) }}</ToneBadge></CardDescription>
            </CardHeader>
            <CardContent>
              <dl class="grid grid-cols-1 gap-1 text-xs">
                <div v-for="[k, v] in configLines('provider', a.provider, a.config)" :key="k" class="flex gap-2">
                  <dt class="text-muted-foreground w-28 shrink-0 truncate" :title="k">{{ k }}</dt>
                  <dd class="min-w-0 truncate font-mono" :title="v">{{ v }}</dd>
                </div>
              </dl>
            </CardContent>
          </Card>
        </div>
      </section>

      <section v-if="preview.notifiers.length" class="grid grid-cols-1 gap-2">
        <h2 class="flex items-center gap-2 text-sm font-medium"><Bell class="text-muted-foreground size-4" />{{ t('legacy.notifiers') }} · {{ preview.notifiers.length }}</h2>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Card v-for="(n, i) in preview.notifiers" :key="i" class="gap-2 py-4">
            <CardHeader class="grid-cols-[auto_1fr] gap-x-3">
              <BrandIcon kind="notifier" :type="n.type" class="row-span-2 size-9" />
              <CardTitle class="truncate" :title="n.name">{{ n.name }}</CardTitle>
              <CardDescription class="flex flex-wrap gap-1.5">
                <ToneBadge tone="primary">{{ meta.notifierName(n.type) }}</ToneBadge>
                <ToneBadge v-if="n.onSuccess">{{ t('legacy.onSuccess') }}</ToneBadge>
                <ToneBadge v-if="n.onFailure">{{ t('legacy.onFailure') }}</ToneBadge>
                <ToneBadge v-if="n.onChangeOnly">{{ t('legacy.onlyOnChange') }}</ToneBadge>
              </CardDescription>
            </CardHeader>
            <CardContent>
              <dl class="grid grid-cols-1 gap-1 text-xs">
                <div v-for="[k, v] in configLines('notifier', n.type, n.config)" :key="k" class="flex gap-2">
                  <dt class="text-muted-foreground w-28 shrink-0 truncate" :title="k">{{ k }}</dt>
                  <dd class="min-w-0 truncate font-mono" :title="v">{{ v }}</dd>
                </div>
              </dl>
            </CardContent>
          </Card>
        </div>
      </section>

      <section v-if="preview.tasks.length" class="grid grid-cols-1 gap-2">
        <h2 class="flex items-center gap-2 text-sm font-medium"><ListChecks class="text-muted-foreground size-4" />{{ t('legacy.tasks') }} · {{ preview.tasks.length }}</h2>
        <Card v-for="(task, i) in preview.tasks" :key="i" class="gap-2 py-4">
          <CardHeader>
            <CardTitle class="truncate" :title="task.name">{{ task.name }}</CardTitle>
            <CardDescription class="flex flex-wrap gap-1.5">
              <ToneBadge>{{ task.cron ? describeCron(task.cron) : t('cron.presets.manual') }}</ToneBadge>
              <ToneBadge>{{ ipTypeLabel[task.ipType as IPType] ?? task.ipType }}</ToneBadge>
            </CardDescription>
          </CardHeader>
          <CardContent class="text-muted-foreground text-sm whitespace-pre-line">{{ task.summary }}</CardContent>
        </Card>
      </section>

      <p v-if="preview.settings?.githubMirror" class="text-muted-foreground text-sm">
        {{ t('legacy.mirror') }}<span class="text-foreground font-mono">{{ preview.settings.githubMirror }}</span>
      </p>

      <div class="flex flex-wrap justify-end gap-2">
        <Button variant="outline" @click="reset">{{ t('legacy.backToEdit') }}</Button>
        <Button :disabled="!total || applying" @click="apply"><Loader2 v-if="applying" class="animate-spin" /><Upload v-else />{{ t('legacy.confirmImport') }}</Button>
      </div>
    </template>

    <!-- 输入 -->
    <Card v-else>
      <CardHeader>
        <CardTitle class="flex items-center gap-2"><ScanText class="text-muted-foreground size-4" />{{ t('legacy.pasteTitle') }}</CardTitle>
        <CardDescription>{{ t('legacy.pasteDescription') }}</CardDescription>
      </CardHeader>
      <CardContent class="grid grid-cols-1 gap-3">
        <div
          :class="['relative rounded-md transition-shadow', dragging ? 'ring-primary ring-2' : '']"
          @dragover.prevent="dragging = true"
          @dragleave="dragging = false"
          @drop.prevent="onDrop"
        >
          <Textarea v-model="content" :placeholder="placeholder" class="min-h-64 font-mono text-code" spellcheck="false" />
        </div>
        <p class="text-muted-foreground text-xs">{{ t('legacy.dropHint') }}</p>
      </CardContent>
      <CardFooter class="flex flex-wrap justify-between gap-2 border-t">
        <Button variant="outline" @click="fileInput?.click()"><FileUp />{{ t('legacy.pickFile') }}</Button>
        <input ref="fileInput" type="file" accept=".sh,.env,.yml,.yaml,.txt,text/plain" class="hidden" @change="onPick" />
        <Button :disabled="parsing" @click="parse"><Loader2 v-if="parsing" class="animate-spin" /><ScanText v-else />{{ t('legacy.parse') }}</Button>
      </CardFooter>
    </Card>
  </div>
</template>
