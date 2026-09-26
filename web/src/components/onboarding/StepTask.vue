<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { CircleCheck, Loader2, Plus, Save } from '@lucide/vue'
import { accountsApi, notifiersApi, tasksApi } from '@/api'
import type { Account, IPType, Notifier, Task, TaskInput } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import FormItem from '@/components/FormItem.vue'
import NumInput from '@/components/NumInput.vue'
import SettingRow from '@/components/SettingRow.vue'
import SuggestInput from '@/components/SuggestInput.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import { cn } from '@/lib/utils'
import { describeCron } from '@/utils/cron'
import { ipTypeLabel } from '@/utils/format'

const emit = defineEmits<{ (e: 'changed'): void }>()

interface Template {
  key: string
  title: string
  description: string
  ipType: IPType
  cron: string
}

const TEMPLATES: Template[] = [
  { key: 'v4', title: 'IPv4 · 每 6 小时', description: '最常用：优选 1 个 IPv4 写入 A 记录', ipType: 'v4', cron: '0 */6 * * *' },
  { key: 'v6', title: 'IPv6 · 每 6 小时', description: '优选 1 个 IPv6 写入 AAAA 记录', ipType: 'v6', cron: '0 */6 * * *' },
  { key: 'both', title: '双栈 · 每 12 小时', description: '同时写入 A 与 AAAA 记录', ipType: 'both', cron: '0 */12 * * *' },
]

const tasks = ref<Task[] | null>(null)
const accounts = ref<Account[]>([])
const notifiers = ref<Notifier[]>([])
const defaults = ref<Task | null>(null)
const adding = ref(false)
const saving = ref(false)
const domains = ref<string[]>([])
const domainsLoading = ref(false)

const form = reactive({
  template: 'v4',
  accountId: 0,
  domain: '',
  rr: '',
  recordCount: 1,
  notify: true,
  name: '',
  nameTouched: false,
})
const errors = reactive<Record<string, string>>({})

const tpl = computed(() => TEMPLATES.find((t) => t.key === form.template) ?? TEMPLATES[0])
const account = computed(() => accounts.value.find((a) => a.id === form.accountId))
const fqdn = computed(() => {
  const d = form.domain.trim()
  const r = form.rr.trim()
  if (!d) return ''
  return !r || r === '@' ? d : `${r}.${d}`
})
const autoName = computed(() => (fqdn.value ? `${fqdn.value} ${ipTypeLabel[tpl.value.ipType] ?? ''}`.trim() : ''))

watch(autoName, (v) => {
  if (!form.nameTouched) form.name = v
})

async function load() {
  const [t, a, n, d] = await Promise.all([
    tasksApi.list().catch(() => []),
    accountsApi.list().catch(() => []),
    notifiersApi.list().catch(() => []),
    tasksApi.defaults().catch(() => null),
  ])
  tasks.value = t ?? []
  accounts.value = a ?? []
  notifiers.value = (n ?? []).filter((x) => x.enabled)
  defaults.value = d
  adding.value = !tasks.value.length
  if (!form.accountId && accounts.value.length) selectAccount(accounts.value[0].id)
}

onMounted(load)

async function selectAccount(id: number) {
  form.accountId = id
  domains.value = []
  domainsLoading.value = true
  try {
    domains.value = (await accountsApi.domains(id)) ?? []
    if (!form.domain && domains.value.length === 1) form.domain = domains.value[0]
  } catch {
    /* 候选加载失败时可手动输入 */
  } finally {
    domainsLoading.value = false
  }
}

function onAccount(v: unknown) {
  const id = Number(v)
  if (id && id !== form.accountId) selectAccount(id)
}

function validate() {
  for (const k of Object.keys(errors)) delete errors[k]
  if (!form.accountId) errors.account = '请先添加 DNS 账号'
  if (!form.domain.trim()) errors.domain = '请填写主域名'
  if (!form.rr.trim()) errors.rr = '请填写主机记录，根域名填 @'
  if (!form.name.trim()) errors.name = '请填写任务名称'
  return !Object.keys(errors).length
}

async function create() {
  if (!validate()) return
  const d = defaults.value
  if (!d) {
    toast.error('无法获取任务默认参数，请刷新后重试')
    return
  }
  const acc = account.value
  const body: TaskInput = {
    name: form.name.trim(),
    enabled: true,
    cron: tpl.value.cron,
    ipType: tpl.value.ipType,
    speedTest: { ...d.speedTest },
    update: { ...d.update, recordCount: form.recordCount || 1 },
    targets: [
      {
        accountId: form.accountId,
        domain: form.domain.trim(),
        rr: form.rr.trim(),
        ttl: acc?.provider === 'cloudflare' ? 1 : 600,
        proxied: false,
        line: '',
      },
    ],
    notifierIds: form.notify ? notifiers.value.map((n) => n.id) : [],
  }
  saving.value = true
  try {
    await tasksApi.create(body)
    toast.success('任务已创建')
    form.nameTouched = false
    await load()
    emit('changed')
  } catch {
    /* 已提示 */
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="grid grid-cols-1 gap-4">
    <Skeleton v-if="!tasks" class="h-48" />
    <template v-else>
      <div v-if="tasks.length" class="grid grid-cols-1 gap-2">
        <div v-for="t in tasks" :key="t.id" class="flex items-center gap-3 rounded-lg border px-3 py-2.5">
          <CircleCheck class="text-success size-4 shrink-0" />
          <router-link :to="`/tasks/${t.id}`" class="min-w-0 flex-1 truncate text-sm font-medium hover:underline" :title="t.name">{{ t.name }}</router-link>
          <span class="text-muted-foreground hidden text-xs sm:inline">{{ t.cron ? describeCron(t.cron) : '仅手动' }}</span>
          <ToneBadge>{{ ipTypeLabel[t.ipType] ?? t.ipType }}</ToneBadge>
        </div>
        <div v-if="!adding">
          <Button variant="outline" size="sm" @click="adding = true"><Plus />再创建一个</Button>
        </div>
      </div>

      <p v-if="adding && !accounts.length" class="text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm">
        需要先在上一步添加 DNS 账号
      </p>

      <div v-else-if="adding" class="grid grid-cols-1 gap-4">
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
          <button
            v-for="t in TEMPLATES"
            :key="t.key"
            type="button"
            :class="
              cn(
                'rounded-lg border p-3 text-left transition-colors',
                form.template === t.key ? 'border-primary bg-primary/5' : 'hover:bg-accent/50',
              )
            "
            @click="form.template = t.key"
          >
            <div class="text-sm font-medium">{{ t.title }}</div>
            <div class="text-muted-foreground mt-0.5 text-xs leading-relaxed">{{ t.description }}</div>
          </button>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <FormItem label="DNS 账号" required :error="errors.account" class="sm:col-span-2">
            <Select :model-value="form.accountId || undefined" @update:model-value="onAccount">
              <SelectTrigger class="w-full"><SelectValue placeholder="选择账号" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="a in accounts" :key="a.id" :value="a.id">{{ a.name }}</SelectItem>
              </SelectContent>
            </Select>
          </FormItem>
          <FormItem label="主域名" required :error="errors.domain" help="账号下托管的域名，如 example.com">
            <SuggestInput v-model="form.domain" :options="domains" :loading="domainsLoading" placeholder="example.com" />
          </FormItem>
          <FormItem label="主机记录" required :error="errors.rr" help="如 cdn；根域名填 @">
            <Input v-model="form.rr" placeholder="cdn" />
          </FormItem>
          <FormItem label="写入 IP 数量" help="大于 1 时创建多条同名记录做负载均衡">
            <NumInput v-model="form.recordCount" :min="1" :max="10" suffix="条" class="max-w-32" />
          </FormItem>
          <FormItem label="任务名称" required :error="errors.name">
            <Input v-model="form.name" maxlength="64" @input="form.nameTouched = true" />
          </FormItem>
        </div>

        <div v-if="notifiers.length" class="rounded-lg border">
          <SettingRow label="发送通知" :description="`推送到已添加的 ${notifiers.length} 个渠道`" for="ob-task-notify">
            <Switch id="ob-task-notify" v-model="form.notify" />
          </SettingRow>
        </div>

        <p class="text-muted-foreground text-xs">
          <template v-if="fqdn">将把优选 IP 写入 <span class="text-foreground font-mono">{{ fqdn }}</span>，</template>
          执行周期为「{{ describeCron(tpl.cron) }}」。其余测速参数使用默认值，之后可以在任务页调整。
        </p>
        <div>
          <Button :disabled="saving" @click="create"><Loader2 v-if="saving" class="animate-spin" /><Save v-else />创建任务</Button>
        </div>
      </div>
    </template>
  </div>
</template>
