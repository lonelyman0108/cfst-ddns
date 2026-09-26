<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Copy, Download, FileInput, HardDrive, KeyRound, Loader2, RefreshCw, Save, Server, Settings2, ShieldAlert, Upload, Webhook } from '@lucide/vue'
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
import { confirm } from '@/composables/useConfirm'
import { copyText, dayjs, fmtTime, fromNow, saveBlob } from '@/utils/format'

const SECTIONS: NavSection[] = [
  { id: 'general', title: '常规', icon: Settings2 },
  { id: 'webhook', title: 'Webhook 触发', icon: Webhook },
  { id: 'security', title: '账号安全', icon: KeyRound },
  { id: 'backup', title: '备份与恢复', icon: HardDrive },
  { id: 'system', title: '系统信息', icon: Server },
]

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
})

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
    toast.success('设置已保存')
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
    toast.success(v ? 'Webhook 触发已启用' : 'Webhook 触发已关闭')
  } catch {
    /* 已提示 */
  } finally {
    hookSaving.value = false
  }
}

async function regenerate() {
  const ok = await confirm({
    title: '重新生成令牌？',
    description: '旧令牌将立即失效，使用旧令牌的外部调用需同步更新。',
    confirmText: '重新生成',
    destructive: true,
  })
  if (!ok) return
  regenerating.value = true
  try {
    applySettings(await settingsApi.regenerateHookToken())
    toast.success('令牌已重新生成')
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
    title: '下载备份',
    description: '备份文件包含所有 DNS 账号与通知渠道的明文凭据，请妥善保管，切勿公开分享。',
    confirmText: '我已了解，下载',
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
    toast.error('文件不是有效的 JSON')
    return
  }
  const ok1 = await confirm({
    title: '从备份恢复？',
    description: `将使用「${file.name}」恢复配置。现有的 DNS 账号、通知渠道、任务与系统设置将被全部覆盖，此操作不可撤销。`,
    confirmText: '继续',
    destructive: true,
  })
  if (!ok1) return
  const ok2 = await confirm({ title: '请再次确认', description: '现有配置将被覆盖。', confirmText: '确认恢复', destructive: true })
  if (!ok2) return
  restoring.value = true
  try {
    await backupApi.restore(data)
    toast.success('配置已恢复')
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
    <PageHeader title="系统设置" description="常规选项、Webhook、备份与系统信息" />

    <SectionNav :sections="SECTIONS">
      <Card id="general" class="scroll-mt-20">
        <CardHeader>
          <CardTitle class="flex items-center gap-2"><Settings2 class="text-muted-foreground size-4" />常规</CardTitle>
        </CardHeader>
        <CardContent class="grid gap-3">
          <Skeleton v-if="!settings" class="h-40" />
          <template v-else>
            <FormItem label="GitHub 镜像" for="mirror" tip="用于加速下载 cfst 安装包（Releases 列表仍直连 GitHub API）。">
              <Input id="mirror" v-model="form.githubMirror" class="font-mono" placeholder="https://ghfast.top/{url}（留空直连 GitHub）" />
              <template #help>
                支持两种写法：<code>https://ghfast.top/{url}</code>（{url} 替换为完整下载地址），或 <code>https://mirror.example</code>（替换 <code>https://github.com</code> 前缀）。留空表示直连。
              </template>
            </FormItem>
            <div class="grid gap-x-4 gap-y-3 sm:grid-cols-2">
              <FormItem label="历史保留天数" help="超过的执行记录会被自动清理，0 表示不清理">
                <NumInput v-model="form.historyRetentionDays" :min="0" :max="3650" class="max-w-40" />
              </FormItem>
              <FormItem label="通知标题前缀" for="prefix" help="多台设备部署时用于区分通知来源">
                <Input id="prefix" v-model="form.notifyTitlePrefix" maxlength="32" placeholder="如 [家里 NAS]" />
              </FormItem>
            </div>
          </template>
        </CardContent>
        <CardFooter class="justify-end border-t">
          <Button :disabled="saving || !settings" @click="saveGeneral"><Loader2 v-if="saving" class="animate-spin" /><Save v-else />保存</Button>
        </CardFooter>
      </Card>

      <Card id="webhook" class="scroll-mt-20">
        <CardHeader>
          <CardTitle class="flex items-center gap-2"><Webhook class="text-muted-foreground size-4" />Webhook 触发</CardTitle>
          <CardDescription>允许外部系统通过 URL 触发任务执行（无需登录）</CardDescription>
        </CardHeader>
        <CardContent class="grid gap-3">
          <label class="flex min-h-11 items-center justify-between gap-4 rounded-md border px-3 py-2">
            <div>
              <div class="text-sm font-medium">启用 Webhook 触发</div>
              <div class="text-muted-foreground text-xs">关闭时所有 Webhook 调用都会被拒绝</div>
            </div>
            <Switch :model-value="settings?.hookEnabled ?? false" :disabled="!settings || hookSaving" @update:model-value="toggleHook" />
          </label>
          <template v-if="settings?.hookEnabled">
            <FormItem label="令牌">
              <div class="flex gap-2">
                <Input :model-value="settings.hookToken" readonly class="font-mono" />
                <Button variant="outline" size="icon" title="复制" @click="copyText(settings.hookToken)"><Copy /></Button>
                <Button variant="outline" :disabled="regenerating" @click="regenerate">
                  <Loader2 v-if="regenerating" class="animate-spin" /><RefreshCw v-else />重新生成
                </Button>
              </div>
            </FormItem>
            <FormItem label="调用示例">
              <div class="bg-term text-term-foreground group relative rounded-lg border border-zinc-800 p-3 pr-11 font-mono text-code break-all">
                {{ curlExample }}
                <button
                  type="button"
                  class="absolute top-2 right-2 rounded p-1.5 text-zinc-400 hover:bg-white/10 hover:text-white"
                  title="复制"
                  @click="copyText(curlExample)"
                >
                  <Copy class="size-3.5" />
                </button>
              </div>
              <template #help>
                将 <code>{taskId}</code> 替换为任务 ID（任务编辑页地址中的数字）。GET 与 POST 均可，返回 <code>{"runId": 123}</code>。
              </template>
            </FormItem>
          </template>
        </CardContent>
      </Card>

      <Card id="security" class="scroll-mt-20">
        <CardHeader>
          <CardTitle class="flex items-center gap-2"><KeyRound class="text-muted-foreground size-4" />账号安全</CardTitle>
          <CardDescription>修改后其他设备上的登录会失效</CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="outline" @click="pwdOpen = true"><KeyRound />修改密码</Button>
        </CardContent>
      </Card>

      <Card id="backup" class="scroll-mt-20">
        <CardHeader>
          <CardTitle class="flex items-center gap-2"><HardDrive class="text-muted-foreground size-4" />备份与恢复</CardTitle>
          <CardDescription>导出或导入全部配置</CardDescription>
        </CardHeader>
        <CardContent class="grid gap-3">
          <Alert class="border-warning/40 bg-warning/5">
            <ShieldAlert class="text-warning!" />
            <AlertDescription>备份文件包含<strong class="text-foreground">明文凭据</strong>，恢复会覆盖现有全部配置。</AlertDescription>
          </Alert>
          <div class="flex flex-wrap gap-2">
            <Button variant="outline" :disabled="downloading" @click="downloadBackup">
              <Loader2 v-if="downloading" class="animate-spin" /><Download v-else />下载备份
            </Button>
            <Button variant="outline" class="text-destructive hover:text-destructive" :disabled="restoring" @click="fileInput?.click()">
              <Loader2 v-if="restoring" class="animate-spin" /><Upload v-else />从备份恢复
            </Button>
            <input ref="fileInput" type="file" accept=".json,application/json" class="hidden" @change="onFile" />
          </div>
          <p class="text-muted-foreground text-xs">
            从 v1（Bash 脚本版）升级？
            <InlineLink to="/import/legacy" :icon="FileInput">从 v1 导入配置</InlineLink>
          </p>
        </CardContent>
      </Card>

      <Card id="system" class="scroll-mt-20">
        <CardHeader>
          <CardTitle class="flex items-center gap-2"><Server class="text-muted-foreground size-4" />系统信息</CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton v-if="!info" class="h-32" />
          <dl v-else class="grid gap-x-8 gap-y-4 text-sm sm:grid-cols-2 lg:grid-cols-3">
            <div><dt class="text-muted-foreground text-xs">版本</dt><dd class="mt-0.5 font-medium">{{ info.version }}</dd></div>
            <div><dt class="text-muted-foreground text-xs">提交</dt><dd class="mt-0.5 font-mono text-code">{{ info.commit || '-' }}</dd></div>
            <div>
              <dt class="text-muted-foreground text-xs">构建时间</dt>
              <dd class="mt-0.5">{{ fmtTime(info.buildTime) !== '-' ? fmtTime(info.buildTime) : info.buildTime || '-' }}</dd>
            </div>
            <div><dt class="text-muted-foreground text-xs">Go 版本</dt><dd class="mt-0.5 font-mono text-code">{{ info.goVersion }}</dd></div>
            <div><dt class="text-muted-foreground text-xs">平台</dt><dd class="mt-0.5 font-mono text-code">{{ info.os }}/{{ info.arch }}</dd></div>
            <div><dt class="text-muted-foreground text-xs">时区</dt><dd class="mt-0.5">{{ info.timezone }}</dd></div>
            <div>
              <dt class="text-muted-foreground text-xs">启动时间</dt>
              <dd class="mt-0.5">{{ fmtTime(info.startedAt) }} <span class="text-muted-foreground text-xs">· {{ fromNow(info.startedAt) }}</span></dd>
            </div>
            <div class="sm:col-span-2"><dt class="text-muted-foreground text-xs">数据目录</dt><dd class="mt-0.5 font-mono text-code break-all">{{ info.dataDir }}</dd></div>
          </dl>
        </CardContent>
      </Card>

    </SectionNav>

    <ChangePasswordDialog v-model="pwdOpen" />
  </div>
</template>
