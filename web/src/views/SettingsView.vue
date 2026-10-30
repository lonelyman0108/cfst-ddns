<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { CircleArrowUp, CircleCheck, CircleX, Copy, Download, ExternalLink, FileInput, HardDrive, KeyRound, Loader2, RefreshCw, Save, Server, Settings2, ShieldAlert, Upload, Webhook } from '@lucide/vue'
import { backupApi, settingsApi, systemApi } from '@/api'
import type { Settings, SystemInfo } from '@/api/types'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import ChangePasswordDialog from '@/components/ChangePasswordDialog.vue'
import FormItem from '@/components/FormItem.vue'
import InlineLink from '@/components/InlineLink.vue'
import NumInput from '@/components/NumInput.vue'
import PageHeader from '@/components/PageHeader.vue'
import SectionNav, { type NavSection } from '@/components/SectionNav.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import { confirm } from '@/composables/useConfirm'
import { useUpdateStore } from '@/stores/update'
import { copyText, dayjs, fmtTime, fromNow, saveBlob } from '@/utils/format'

const { t } = useI18n()

const SECTIONS = computed<NavSection[]>(() => [
  { id: 'general', title: t('settings.general.title'), icon: Settings2 },
  { id: 'webhook', title: t('settings.webhook.title'), icon: Webhook },
  { id: 'security', title: t('settings.security.title'), icon: KeyRound },
  { id: 'backup', title: t('settings.backup.title'), icon: HardDrive },
  { id: 'system', title: t('settings.system.title'), icon: Server },
])

const settings = ref<Settings | null>(null)
const form = reactive({ githubMirror: '', historyRetentionDays: 30 as number | undefined, notifyTitlePrefix: '' })
const saving = ref(false)
const hookSaving = ref(false)
const regenerating = ref(false)
const pwdOpen = ref(false)
const info = ref<SystemInfo | null>(null)

function applySettings(s: Settings) {
  settings.value = s
  form.githubMirror = s.githubMirror ?? ''
  form.historyRetentionDays = s.historyRetentionDays ?? 30
  form.notifyTitlePrefix = s.notifyTitlePrefix ?? ''
}

async function load() {
  try {
    applySettings(await settingsApi.get())
  } catch {
    /* 已提示 */
  }
}

onMounted(() => {
  load()
  systemApi
    .info()
    .then((r) => (info.value = r))
    .catch(() => {})
  update.load().catch(() => {})
})

// ---------- 版本与更新 ----------
const update = useUpdateStore()
const updateSaving = ref(false)

const commitUrl = computed(() => {
  const i = info.value
  return i && /^[0-9a-f]{7,40}$/.test(i.commit) ? `${i.repoUrl}/commit/${i.commit}` : ''
})
const runUrl = computed(() => (info.value?.runId ? `${info.value.repoUrl}/actions/runs/${info.value.runId}` : ''))

async function toggleUpdateCheck(v: boolean) {
  updateSaving.value = true
  try {
    applySettings(await settingsApi.update({ updateCheck: v }))
    if (v) update.load().catch(() => {})
  } catch {
    /* 已提示 */
  } finally {
    updateSaving.value = false
  }
}

function checkNow() {
  update.load(true).catch(() => {})
}

async function saveGeneral() {
  saving.value = true
  try {
    applySettings(
      await settingsApi.update({
        githubMirror: form.githubMirror.trim(),
        historyRetentionDays: form.historyRetentionDays ?? 30,
        notifyTitlePrefix: form.notifyTitlePrefix,
      }),
    )
    toast.success(t('settings.saved'))
  } catch {
    /* 已提示 */
  } finally {
    saving.value = false
  }
}

async function toggleHook(v: boolean) {
  hookSaving.value = true
  try {
    applySettings(await settingsApi.update({ hookEnabled: v }))
    toast.success(v ? t('settings.webhook.enabledToast') : t('settings.webhook.disabledToast'))
  } catch {
    /* 已提示 */
  } finally {
    hookSaving.value = false
  }
}

async function regenerate() {
  const ok = await confirm({
    title: t('settings.webhook.regenerateTitle'),
    description: t('settings.webhook.regenerateDescription'),
    confirmText: t('settings.webhook.regenerate'),
    destructive: true,
  })
  if (!ok) return
  regenerating.value = true
  try {
    applySettings(await settingsApi.regenerateHookToken())
    toast.success(t('settings.webhook.regenerated'))
  } catch {
    /* 已提示 */
  } finally {
    regenerating.value = false
  }
}

const hookUrl = computed(() => `${window.location.origin}/api/hooks/tasks/{taskId}/run?token=${settings.value?.hookToken ?? ''}`)
const curlExample = computed(() => `curl -X POST "${hookUrl.value}"`)

// ---------- 备份与恢复 ----------
const downloading = ref(false)
const restoring = ref(false)
const fileInput = ref<HTMLInputElement>()

async function downloadBackup() {
  const ok = await confirm({
    title: t('settings.backup.download'),
    description: t('settings.backup.downloadDescription'),
    confirmText: t('settings.backup.downloadConfirm'),
  })
  if (!ok) return
  downloading.value = true
  try {
    const { blob, filename } = await backupApi.download()
    saveBlob(blob, filename || `cfst-ddns-backup-${dayjs().format('YYYYMMDD-HHmmss')}.json`)
  } catch {
    /* 已提示 */
  } finally {
    downloading.value = false
  }
}

async function onFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  let data: unknown
  try {
    data = JSON.parse(await file.text())
  } catch {
    toast.error(t('settings.backup.invalidJson'))
    return
  }
  const ok1 = await confirm({
    title: t('settings.backup.restoreTitle'),
    description: t('settings.backup.restoreDescription', { file: file.name }),
    confirmText: t('settings.backup.continue'),
    destructive: true,
  })
  if (!ok1) return
  const ok2 = await confirm({
    title: t('settings.backup.confirmAgainTitle'),
    description: t('settings.backup.confirmAgainDescription'),
    confirmText: t('settings.backup.confirmRestore'),
    destructive: true,
  })
  if (!ok2) return
  restoring.value = true
  try {
    await backupApi.restore(data)
    toast.success(t('settings.backup.restored'))
    load()
  } catch {
    /* 已提示 */
  } finally {
    restoring.value = false
  }
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-5xl flex-col gap-4">
    <PageHeader :title="t('settings.title')" :description="t('settings.description')" />

    <SectionNav :sections="SECTIONS">
      <Card id="general" class="scroll-mt-20">
        <CardHeader>
          <CardTitle class="flex items-center gap-2"><Settings2 class="text-muted-foreground size-4" />{{ t('settings.general.title') }}</CardTitle>
        </CardHeader>
        <CardContent class="grid grid-cols-1 gap-3">
          <Skeleton v-if="!settings" class="h-40" />
          <template v-else>
            <FormItem :label="t('settings.general.mirror')" for="mirror">
              <Input id="mirror" v-model="form.githubMirror" class="font-mono" :placeholder="t('settings.general.mirrorPlaceholder', { url: '{url}' })" />
              <template #help>
                <i18n-t keypath="settings.general.mirrorHelp">
                  <template #format><code>https://ghfast.top/{url}</code></template>
                  <template #token>{url}</template>
                  <template #mirror><code>https://mirror.example</code></template>
                  <template #github><code>https://github.com</code></template>
                </i18n-t>
              </template>
            </FormItem>
            <div class="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2">
              <FormItem :label="t('settings.general.retention')" :help="t('settings.general.retentionHelp')">
                <NumInput v-model="form.historyRetentionDays" :min="0" :max="3650" class="max-w-40" />
              </FormItem>
              <FormItem :label="t('settings.general.titlePrefix')" for="prefix" :help="t('settings.general.titlePrefixHelp')">
                <Input id="prefix" v-model="form.notifyTitlePrefix" maxlength="32" :placeholder="t('settings.general.titlePrefixPlaceholder')" />
              </FormItem>
            </div>
          </template>
        </CardContent>
        <CardFooter class="justify-end border-t">
          <Button :disabled="saving || !settings" @click="saveGeneral"><Loader2 v-if="saving" class="animate-spin" /><Save v-else />{{ t('common.save') }}</Button>
        </CardFooter>
      </Card>

      <Card id="webhook" class="scroll-mt-20">
        <CardHeader>
          <CardTitle class="flex items-center gap-2"><Webhook class="text-muted-foreground size-4" />{{ t('settings.webhook.title') }}</CardTitle>
          <CardDescription>{{ t('settings.webhook.description') }}</CardDescription>
        </CardHeader>
        <CardContent class="grid grid-cols-1 gap-3">
          <label class="flex min-h-11 items-center justify-between gap-4 rounded-md border px-3 py-2">
            <div>
              <div class="text-sm font-medium">{{ t('settings.webhook.enable') }}</div>
              <div class="text-muted-foreground text-xs">{{ t('settings.webhook.enableHelp') }}</div>
            </div>
            <Switch :model-value="settings?.hookEnabled ?? false" :disabled="!settings || hookSaving" @update:model-value="toggleHook" />
          </label>
          <template v-if="settings?.hookEnabled">
            <FormItem :label="t('settings.webhook.token')">
              <div class="flex gap-2">
                <Input :model-value="settings.hookToken" readonly class="font-mono" />
                <Button variant="outline" size="icon" :title="t('common.copy')" @click="copyText(settings.hookToken)"><Copy /></Button>
                <Button variant="outline" :disabled="regenerating" @click="regenerate">
                  <Loader2 v-if="regenerating" class="animate-spin" /><RefreshCw v-else />{{ t('settings.webhook.regenerate') }}
                </Button>
              </div>
            </FormItem>
            <FormItem :label="t('settings.webhook.example')">
              <div class="bg-term text-term-foreground group relative rounded-lg border border-zinc-800 p-3 pr-11 font-mono text-code break-all">
                {{ curlExample }}
                <button
                  type="button"
                  class="absolute top-2 right-2 rounded p-1.5 text-zinc-400 hover:bg-white/10 hover:text-white"
                  :title="t('common.copy')"
                  @click="copyText(curlExample)"
                >
                  <Copy class="size-3.5" />
                </button>
              </div>
              <template #help>
                <i18n-t keypath="settings.webhook.exampleHelp">
                  <template #taskId><code>{taskId}</code></template>
                  <template #result><code>{"runId": 123}</code></template>
                </i18n-t>
              </template>
            </FormItem>
          </template>
        </CardContent>
      </Card>

      <Card id="security" class="scroll-mt-20">
        <CardHeader>
          <CardTitle class="flex items-center gap-2"><KeyRound class="text-muted-foreground size-4" />{{ t('settings.security.title') }}</CardTitle>
          <CardDescription>{{ t('settings.security.description') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="outline" @click="pwdOpen = true"><KeyRound />{{ t('components.changePassword.title') }}</Button>
        </CardContent>
      </Card>

      <Card id="backup" class="scroll-mt-20">
        <CardHeader>
          <CardTitle class="flex items-center gap-2"><HardDrive class="text-muted-foreground size-4" />{{ t('settings.backup.title') }}</CardTitle>
          <CardDescription>{{ t('settings.backup.description') }}</CardDescription>
        </CardHeader>
        <CardContent class="grid grid-cols-1 gap-3">
          <Alert class="border-warning/40 bg-warning/5">
            <ShieldAlert class="text-warning!" />
            <AlertDescription>
              <i18n-t keypath="settings.backup.warning">
                <template #credentials><strong class="text-foreground">{{ t('settings.backup.plaintext') }}</strong></template>
              </i18n-t>
            </AlertDescription>
          </Alert>
          <div class="flex flex-wrap gap-2">
            <Button variant="outline" :disabled="downloading" @click="downloadBackup">
              <Loader2 v-if="downloading" class="animate-spin" /><Download v-else />{{ t('settings.backup.download') }}
            </Button>
            <Button variant="outline" class="text-destructive hover:text-destructive" :disabled="restoring" @click="fileInput?.click()">
              <Loader2 v-if="restoring" class="animate-spin" /><Upload v-else />{{ t('settings.backup.restore') }}
            </Button>
            <input ref="fileInput" type="file" accept=".json,application/json" class="hidden" @change="onFile" />
          </div>
          <p class="text-muted-foreground text-xs">
            {{ t('settings.backup.legacyHint') }}
            <InlineLink to="/import/legacy" :icon="FileInput">{{ t('settings.backup.legacyLink') }}</InlineLink>
          </p>
        </CardContent>
      </Card>

      <Card id="system" class="scroll-mt-20">
        <CardHeader>
          <CardTitle class="flex items-center gap-2"><Server class="text-muted-foreground size-4" />{{ t('settings.system.title') }}</CardTitle>
        </CardHeader>
        <CardContent class="grid grid-cols-1 gap-4">
          <div class="divide-y rounded-md border">
            <label class="flex min-h-11 items-center justify-between gap-4 px-3 py-2">
              <div class="min-w-0">
                <div class="text-sm font-medium">{{ t('settings.system.updateCheck') }}</div>
                <div class="text-muted-foreground text-xs">{{ t('settings.system.updateCheckHelp') }}</div>
              </div>
              <Switch :model-value="settings?.updateCheck ?? true" :disabled="!settings || updateSaving" @update:model-value="toggleUpdateCheck" />
            </label>
            <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-3 py-2 text-sm">
              <div v-if="update.info?.channel === 'dev'" class="text-muted-foreground">{{ t('settings.system.updateDev') }}</div>
              <div v-else-if="update.info?.available" class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                <CircleArrowUp class="text-primary size-4 shrink-0" />
                <span class="font-medium">{{ t('settings.system.updateAvailable', { version: update.info.latest }) }}</span>
                <span v-if="update.info.publishedAt" class="text-muted-foreground text-xs">{{ t('settings.system.publishedAt', { time: fromNow(update.info.publishedAt) }) }}</span>
                <InlineLink v-if="update.info.url" :href="update.info.url" :icon="ExternalLink">{{ t('settings.system.releaseNotes') }}</InlineLink>
              </div>
              <div v-else-if="update.info?.error" class="flex min-w-0 items-start gap-2">
                <CircleX class="text-destructive mt-0.5 size-4 shrink-0" />
                <span class="min-w-0 wrap-anywhere"><span class="text-destructive">{{ t('settings.system.updateFailed') }}</span> <span class="text-muted-foreground text-xs">{{ update.info.error }}</span></span>
              </div>
              <div v-else-if="update.info?.checkedAt" class="flex items-center gap-2">
                <CircleCheck class="text-success size-4 shrink-0" />{{ t('settings.system.upToDate') }}
                <span class="text-muted-foreground text-xs">{{ t('settings.system.checkedAt', { time: fromNow(update.info.checkedAt) }) }}</span>
              </div>
              <div v-else class="text-muted-foreground">{{ t('settings.system.notChecked') }}</div>
              <Button v-if="update.info?.channel !== 'dev'" variant="outline" size="sm" :disabled="update.checking" @click="checkNow">
                <RefreshCw :class="update.checking && 'animate-spin'" />{{ t('settings.system.checkNow') }}
              </Button>
            </div>
          </div>

          <Skeleton v-if="!info" class="h-32" />
          <dl v-else class="grid grid-cols-1 gap-x-8 gap-y-4 text-sm sm:grid-cols-2 lg:grid-cols-3">
            <div><dt class="text-muted-foreground text-xs">{{ t('settings.system.version') }}</dt><dd class="mt-0.5 flex flex-wrap items-center gap-1.5 font-medium">
                <span class="wrap-anywhere">{{ info.version === 'dev' ? t('settings.system.devBuild') : info.version }}</span>
                <ToneBadge v-if="update.info?.channel === 'nightly'" tone="warning">Nightly</ToneBadge>
              </dd></div>
            <div><dt class="text-muted-foreground text-xs">{{ t('settings.system.commit') }}</dt><dd class="mt-0.5 font-mono text-code">
                <InlineLink v-if="commitUrl" :href="commitUrl">{{ info.commit }}</InlineLink>
                <template v-else>{{ info.commit && info.commit !== 'none' ? info.commit : '-' }}</template>
              </dd></div>
            <div><dt class="text-muted-foreground text-xs">{{ t('settings.system.build') }}</dt><dd class="mt-0.5 font-mono text-code">
                <InlineLink v-if="runUrl" :href="runUrl">#{{ info.buildId || info.runId }}</InlineLink>
                <template v-else>{{ info.buildId ? `#${info.buildId}` : '-' }}</template>
              </dd></div>
            <div>
              <dt class="text-muted-foreground text-xs">{{ t('settings.system.buildTime') }}</dt>
              <dd class="mt-0.5">{{ fmtTime(info.buildTime) }}</dd>
            </div>
            <div><dt class="text-muted-foreground text-xs">{{ t('settings.system.goVersion') }}</dt><dd class="mt-0.5 font-mono text-code">{{ info.goVersion }}</dd></div>
            <div><dt class="text-muted-foreground text-xs">{{ t('settings.system.platform') }}</dt><dd class="mt-0.5 font-mono text-code">{{ info.os }}/{{ info.arch }}</dd></div>
            <div><dt class="text-muted-foreground text-xs">{{ t('settings.system.timezone') }}</dt><dd class="mt-0.5">{{ info.timezone }}</dd></div>
            <div>
              <dt class="text-muted-foreground text-xs">{{ t('settings.system.startedAt') }}</dt>
              <dd class="mt-0.5">{{ fmtTime(info.startedAt) }} <span class="text-muted-foreground text-xs">· {{ fromNow(info.startedAt) }}</span></dd>
            </div>
            <div class="sm:col-span-2"><dt class="text-muted-foreground text-xs">{{ t('settings.system.dataDir') }}</dt><dd class="mt-0.5 font-mono text-code break-all">{{ info.dataDir }}</dd></div>
          </dl>
        </CardContent>
      </Card>

    </SectionNav>

    <ChangePasswordDialog v-model="pwdOpen" />
  </div>
</template>
