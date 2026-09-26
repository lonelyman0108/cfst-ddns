<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { useI18n } from 'vue-i18n'
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
const { t } = useI18n()

// 标题与说明见 onboarding.task.templates.<key>
interface Template {
  key: string
  ipType: IPType
  cron: string
}

const TEMPLATES: Template[] = [
  { key: 'v4', ipType: 'v4', cron: '0 */6 * * *' },
  { key: 'v6', ipType: 'v6', cron: '0 */6 * * *' },
  { key: 'both', ipType: 'both', cron: '0 */12 * * *' },
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
// 存文案 key，渲染时 t()
const errors = reactive<Record<string, string>>({})

const tpl = computed(() => TEMPLATES.find((x) => x.key === form.template) ?? TEMPLATES[0])
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
  if (!form.accountId) errors.account = 'onboarding.task.errors.account'
  if (!form.domain.trim()) errors.domain = 'onboarding.task.errors.domain'
  if (!form.rr.trim()) errors.rr = 'onboarding.task.errors.rr'
  if (!form.name.trim()) errors.name = 'onboarding.task.errors.name'
  return !Object.keys(errors).length
}

async function create() {
  if (!validate()) return
  const d = defaults.value
  if (!d) {
    toast.error(t('onboarding.task.noDefaults'))
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
    toast.success(t('onboarding.task.created'))
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
        <div v-for="x in tasks" :key="x.id" class="flex items-center gap-3 rounded-lg border px-3 py-2.5">
          <CircleCheck class="text-success size-4 shrink-0" />
          <router-link :to="`/tasks/${x.id}`" class="min-w-0 flex-1 truncate text-sm font-medium hover:underline" :title="x.name">{{ x.name }}</router-link>
          <span class="text-muted-foreground hidden text-xs sm:inline">{{ x.cron ? describeCron(x.cron) : t('cron.presets.manual') }}</span>
          <ToneBadge>{{ ipTypeLabel[x.ipType] ?? x.ipType }}</ToneBadge>
        </div>
        <div v-if="!adding">
          <Button variant="outline" size="sm" @click="adding = true"><Plus />{{ t('onboarding.task.createAnother') }}</Button>
        </div>
      </div>

      <p v-if="adding && !accounts.length" class="text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm">
        {{ t('onboarding.task.needAccount') }}
      </p>

      <div v-else-if="adding" class="grid grid-cols-1 gap-4">
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
          <button
            v-for="x in TEMPLATES"
            :key="x.key"
            type="button"
            :class="
              cn(
                'rounded-lg border p-3 text-left transition-colors',
                form.template === x.key ? 'border-primary bg-primary/5' : 'hover:bg-accent/50',
              )
            "
            @click="form.template = x.key"
          >
            <div class="text-sm font-medium">{{ t(`onboarding.task.templates.${x.key}.title`) }}</div>
            <div class="text-muted-foreground mt-0.5 text-xs leading-relaxed">{{ t(`onboarding.task.templates.${x.key}.description`) }}</div>
          </button>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <FormItem :label="t('onboarding.task.account')" required :error="errors.account && t(errors.account)" class="sm:col-span-2">
            <Select :model-value="form.accountId || undefined" @update:model-value="onAccount">
              <SelectTrigger class="w-full"><SelectValue :placeholder="t('onboarding.task.selectAccount')" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="a in accounts" :key="a.id" :value="a.id">{{ a.name }}</SelectItem>
              </SelectContent>
            </Select>
          </FormItem>
          <FormItem :label="t('onboarding.task.domain')" required :error="errors.domain && t(errors.domain)" :help="t('onboarding.task.domainHelp')">
            <SuggestInput v-model="form.domain" :options="domains" :loading="domainsLoading" placeholder="example.com" />
          </FormItem>
          <FormItem :label="t('onboarding.task.rr')" required :error="errors.rr && t(errors.rr)" :help="t('onboarding.task.rrHelp')">
            <Input v-model="form.rr" placeholder="cdn" />
          </FormItem>
          <FormItem :label="t('onboarding.task.recordCount')" :help="t('onboarding.task.recordCountHelp')">
            <NumInput v-model="form.recordCount" :min="1" :max="10" :suffix="t('onboarding.task.recordUnit')" class="max-w-32" />
          </FormItem>
          <FormItem :label="t('onboarding.task.name')" required :error="errors.name && t(errors.name)">
            <Input v-model="form.name" maxlength="64" @input="form.nameTouched = true" />
          </FormItem>
        </div>

        <div v-if="notifiers.length" class="rounded-lg border">
          <SettingRow :label="t('onboarding.task.notify')" :description="t('onboarding.task.notifyDesc', { n: notifiers.length })" for="ob-task-notify">
            <Switch id="ob-task-notify" v-model="form.notify" />
          </SettingRow>
        </div>

        <p class="text-muted-foreground text-xs">
          <i18n-t v-if="fqdn" keypath="onboarding.task.summaryTarget">
            <template #fqdn><span class="text-foreground font-mono">{{ fqdn }}</span></template>
          </i18n-t>
          {{ t('onboarding.task.summarySchedule', { cron: describeCron(tpl.cron) }) }}
        </p>
        <div>
          <Button :disabled="saving" @click="create"><Loader2 v-if="saving" class="animate-spin" /><Save v-else />{{ t('onboarding.task.create') }}</Button>
        </div>
      </div>
    </template>
  </div>
</template>
