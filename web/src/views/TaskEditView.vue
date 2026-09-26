<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import {
  ArrowLeft,
  Bell,
  ChevronRight,
  CircleX,
  Gauge,
  Globe,
  Loader2,
  Plus,
  Save,
  Search,
  Settings2,
  SlidersHorizontal,
  Trash2,
  TriangleAlert,
  UserRoundKey,
} from '@lucide/vue'
import { accountsApi, notifiersApi, tasksApi } from '@/api'
import type { Account, DNSRecord, IPType, Notifier, SpeedTestConfig, Target, TaskInput } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle, CardAction } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import BrandIcon from '@/components/BrandIcon.vue'
import CronInput from '@/components/CronInput.vue'
import EmptyState from '@/components/EmptyState.vue'
import FormItem from '@/components/FormItem.vue'
import NumInput from '@/components/NumInput.vue'
import SettingRow from '@/components/SettingRow.vue'
import SuggestInput from '@/components/SuggestInput.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import { confirm } from '@/composables/useConfirm'
import { useMetaStore } from '@/stores/meta'
import { describeCron } from '@/utils/cron'
import { fmtTime, fromNow, ipTypeLabel } from '@/utils/format'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const meta = useMetaStore()

const taskId = computed(() => (route.params.id ? Number(route.params.id) : 0))
const isNew = computed(() => !taskId.value)

const loading = ref(true)
const loadFailed = ref(false)
const saving = ref(false)
const accounts = ref<Account[]>([])
const notifiers = ref<Notifier[]>([])
const advancedOpen = ref(false)
const nextRuns = ref<string[]>([])

const DEFAULT_SPEED: SpeedTestConfig = {
  threads: 200,
  pingTimes: 4,
  downloadCount: 10,
  downloadTime: 10,
  port: 443,
  url: '',
  httping: false,
  httpingCode: 0,
  cfColo: '',
  maxLatency: 9999,
  minLatency: 0,
  maxLossRate: 1,
  minSpeed: 0,
  disableDownload: false,
  allIP: false,
  ipSource: 'default',
  ipv4Ranges: '',
  ipv6Ranges: '',
  extraArgs: '',
}

const form = reactive<TaskInput>({
  name: '',
  enabled: true,
  cron: '',
  ipType: 'v4',
  speedTest: { ...DEFAULT_SPEED },
  update: { recordCount: 1, skipUnchanged: true },
  targets: [],
  notifierIds: [],
})

let snapshot = ''
const snap = () => JSON.stringify(form)

// ---------- 加载 ----------
async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const [task, accs, nts] = await Promise.all([
      isNew.value ? tasksApi.defaults() : tasksApi.get(taskId.value),
      accountsApi.list(),
      notifiersApi.list(),
      meta.loadProviders().catch(() => {}),
      meta.loadNotifiers().catch(() => {}),
    ])
    accounts.value = accs ?? []
    notifiers.value = nts ?? []
    form.name = task.name ?? ''
    form.enabled = task.enabled ?? true
    form.cron = task.cron ?? ''
    form.ipType = task.ipType || 'v4'
    form.speedTest = { ...DEFAULT_SPEED, ...(task.speedTest ?? {}) }
    form.update = Object.assign({ recordCount: 1, skipUnchanged: true }, task.update ?? {})
    form.targets = (task.targets ?? []).map((t) => ({ ...newTarget(), ...t }))
    form.notifierIds = [...(task.notifierIds ?? [])]
    if (isNew.value && !form.targets.length && accounts.value.length) form.targets.push(newTarget())
    form.targets.forEach((t) => t.accountId && loadDomains(t.accountId))
    advancedOpen.value = !!form.speedTest.extraArgs
    snapshot = snap()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

onMounted(load)

onBeforeRouteLeave(async () => {
  if (loading.value || loadFailed.value || saving.value || snap() === snapshot) return true
  return confirm({ title: t('tasks.edit.leaveTitle'), description: t('tasks.edit.leaveDesc'), confirmText: t('tasks.edit.leave'), destructive: true })
})

// ---------- 目标记录 ----------
function newTarget(): Target {
  const acc = accounts.value[0]
  return {
    accountId: acc?.id ?? 0,
    domain: '',
    rr: '',
    ttl: acc?.provider === 'cloudflare' ? 1 : 600,
    proxied: false,
    line: '',
  }
}

function addTarget() {
  const t = newTarget()
  form.targets.push(t)
  if (t.accountId) loadDomains(t.accountId)
}

const accountById = (id: number) => accounts.value.find((a) => a.id === id)
const isCloudflare = (id: number) => accountById(id)?.provider === 'cloudflare'

function onAccountChange(t: Target, v: unknown) {
  t.accountId = Number(v)
  if (isCloudflare(t.accountId)) {
    t.line = ''
  } else {
    t.proxied = false
    if (t.ttl === 1) t.ttl = 600
  }
  if (t.accountId) loadDomains(t.accountId)
}

function fqdn(t: Target) {
  const d = t.domain.trim().replace(/\.$/, '')
  const rr = t.rr.trim()
  if (!d) return ''
  if (!rr || rr === '@') return d
  return `${rr}.${d}`
}

const domains = reactive<Record<number, { loading: boolean; list: string[]; failed: boolean }>>({})

async function loadDomains(accountId: number) {
  const cur = domains[accountId]
  if (cur && (cur.loading || cur.list.length || cur.failed)) return
  domains[accountId] = { loading: true, list: [], failed: false }
  try {
    domains[accountId] = { loading: false, list: (await accountsApi.domains(accountId)) ?? [], failed: false }
  } catch {
    domains[accountId] = { loading: false, list: [], failed: true }
  }
}

const rec = reactive({ open: false, loading: false, title: '', records: [] as DNSRecord[], error: false })

async function viewRecords(tg: Target) {
  if (!tg.accountId || !tg.domain.trim()) {
    toast.warning(t('tasks.edit.pickAccountFirst'))
    return
  }
  Object.assign(rec, { open: true, loading: true, records: [], error: false, title: fqdn(tg) || tg.domain })
  try {
    rec.records = (await accountsApi.records(tg.accountId, tg.domain.trim(), tg.rr.trim() || undefined)) ?? []
  } catch {
    rec.error = true
  } finally {
    rec.loading = false
  }
}

// ---------- 其他 ----------
const showV4 = computed(() => form.ipType === 'v4' || form.ipType === 'both')
const showV6 = computed(() => form.ipType === 'v6' || form.ipType === 'both')

function setIpType(v: unknown) {
  form.ipType = (v as IPType) || 'v4'
}

function toggleNotifier(id: number, on: boolean) {
  const s = new Set(form.notifierIds)
  if (on) s.add(id)
  else s.delete(id)
  form.notifierIds = [...s]
}

/** 摘要：将写入的记录（按 IP 类型展开为 A / AAAA） */
const summaryRecords = computed(() => {
  const types = form.ipType === 'both' ? ['A', 'AAAA'] : form.ipType === 'v6' ? ['AAAA'] : ['A']
  const out: { key: string; type: string; fqdn: string }[] = []
  form.targets.forEach((t, i) => {
    const f = fqdn(t)
    if (f) for (const ty of types) out.push({ key: `${i}-${ty}`, type: ty, fqdn: f })
  })
  return out
})


// ---------- 保存 ----------
function validate(): string {
  if (!form.name.trim()) return t('tasks.edit.v.name')
  if (!form.targets.length) return t('tasks.edit.v.targets')
  for (const [i, tg] of form.targets.entries()) {
    const n = i + 1
    if (!tg.accountId) return t('tasks.edit.v.account', { n })
    if (!tg.domain.trim()) return t('tasks.edit.v.domain', { n })
    if (!tg.rr.trim()) return t('tasks.edit.v.rr', { n })
  }
  if (form.update.recordCount < 1 || form.update.recordCount > 10) return t('tasks.edit.v.recordCount')
  if (form.speedTest.ipSource === 'custom') {
    if (showV4.value && !form.speedTest.ipv4Ranges.trim()) return t('tasks.edit.v.ipv4Ranges')
    if (showV6.value && !form.speedTest.ipv6Ranges.trim()) return t('tasks.edit.v.ipv6Ranges')
  }
  return ''
}

const n = (v: number | undefined, d: number) => (typeof v === 'number' && Number.isFinite(v) ? v : d)

async function save() {
  const err = validate()
  if (err) {
    toast.warning(err)
    return
  }
  const st = form.speedTest
  const body: TaskInput = {
    name: form.name.trim(),
    enabled: form.enabled,
    cron: form.cron.trim(),
    ipType: form.ipType,
    speedTest: {
      ...st,
      threads: n(st.threads, DEFAULT_SPEED.threads),
      pingTimes: n(st.pingTimes, DEFAULT_SPEED.pingTimes),
      downloadCount: n(st.downloadCount, DEFAULT_SPEED.downloadCount),
      downloadTime: n(st.downloadTime, DEFAULT_SPEED.downloadTime),
      port: n(st.port, DEFAULT_SPEED.port),
      httpingCode: n(st.httpingCode, 0),
      maxLatency: n(st.maxLatency, DEFAULT_SPEED.maxLatency),
      minLatency: n(st.minLatency, 0),
      maxLossRate: n(st.maxLossRate, 1),
      minSpeed: n(st.minSpeed, 0),
    },
    update: { recordCount: n(form.update.recordCount, 1), skipUnchanged: form.update.skipUnchanged },
    targets: form.targets.map((t) => ({
      accountId: t.accountId,
      domain: t.domain.trim().replace(/\.$/, ''),
      rr: t.rr.trim(),
      ttl: n(t.ttl, 600),
      proxied: isCloudflare(t.accountId) ? t.proxied : false,
      line: isCloudflare(t.accountId) ? '' : t.line.trim(),
    })),
    notifierIds: [...form.notifierIds],
  }
  saving.value = true
  try {
    if (isNew.value) await tasksApi.create(body)
    else await tasksApi.update(taskId.value, body)
    toast.success(t('tasks.edit.saved'))
    snapshot = snap()
    router.push('/tasks')
  } catch {
    /* 已提示 */
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="mx-auto w-full max-w-[1320px]">
    <!-- 页头：返回、标题、启用开关（取消 / 保存只在吸底栏） -->
    <div class="mb-4 flex flex-wrap items-center gap-3">
      <Button variant="ghost" size="icon" class="size-8" @click="router.push('/tasks')"><ArrowLeft /></Button>
      <div class="min-w-0 flex-1">
        <h1 class="page-title truncate">{{ isNew ? $t('tasks.newTask') : form.name || $t('tasks.edit.editTitle') }}</h1>
        <p class="text-muted-foreground mt-0.5">{{ $t('tasks.edit.subtitle') }}</p>
      </div>
      <label v-if="!loading && !loadFailed" class="flex cursor-pointer items-center gap-2 text-sm font-medium">
        <Switch v-model="form.enabled" />{{ form.enabled ? $t('common.enabled') : $t('common.disabled') }}
      </label>
    </div>

    <div v-if="loading" class="grid grid-cols-1 max-w-5xl gap-4">
      <Skeleton v-for="i in 4" :key="i" class="h-40 rounded-xl" />
    </div>

    <EmptyState v-else-if="loadFailed" :icon="CircleX" :title="$t('tasks.edit.loadFailed')" :description="$t('tasks.edit.loadFailedDesc')">
      <Button variant="outline" size="sm" @click="load">{{ $t('common.retry') }}</Button>
    </EmptyState>

    <div v-else class="grid grid-cols-1 items-start gap-4 xl:grid-cols-[minmax(0,1fr)_280px]">
      <form class="grid grid-cols-1 min-w-0 gap-4" @submit.prevent="save">
        <!-- 基本信息 -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2"><Settings2 class="text-muted-foreground size-4" />{{ $t('tasks.edit.basic') }}</CardTitle>
          </CardHeader>
          <CardContent class="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-3">
            <FormItem :label="$t('tasks.edit.name')" required for="name" class="md:col-span-2">
              <Input id="name" v-model="form.name" maxlength="64" :placeholder="$t('tasks.edit.namePlaceholder')" />
            </FormItem>
            <FormItem :label="$t('tasks.ipType')" :help="$t('tasks.edit.ipTypeHelp')">
              <Tabs :model-value="form.ipType" @update:model-value="setIpType">
                <TabsList class="h-9 w-full">
                  <TabsTrigger value="v4">IPv4</TabsTrigger>
                  <TabsTrigger value="v6">IPv6</TabsTrigger>
                  <TabsTrigger value="both">{{ $t('tasks.edit.dualStack') }}</TabsTrigger>
                </TabsList>
              </Tabs>
            </FormItem>
            <FormItem :label="$t('tasks.schedule')" class="md:col-span-3">
              <CronInput v-model="form.cron" @next="nextRuns = $event" />
            </FormItem>
          </CardContent>
        </Card>

        <!-- 测速参数 -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2"><Gauge class="text-muted-foreground size-4" />{{ $t('tasks.edit.speedTest') }}</CardTitle>
            <CardDescription>{{ $t('tasks.edit.speedTestDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="grid grid-cols-1 gap-4">
            <!-- 延迟测速 -->
            <section class="grid grid-cols-1 gap-2">
              <h4 class="text-muted-foreground text-xs font-medium">{{ $t('tasks.edit.latencyTest') }}</h4>
              <div class="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2 lg:grid-cols-4">
                <FormItem :label="$t('tasks.edit.threads')" flag="-n" :help="$t('tasks.edit.threadsHelp')"><NumInput v-model="form.speedTest.threads" :min="1" :max="1000" /></FormItem>
                <FormItem :label="$t('tasks.edit.pingTimes')" flag="-t" :help="$t('tasks.edit.pingTimesHelp')"><NumInput v-model="form.speedTest.pingTimes" :min="1" :max="100" :suffix="$t('tasks.edit.timesUnit')" /></FormItem>
                <FormItem :label="$t('tasks.edit.port')" flag="-tp" :help="$t('tasks.edit.portHelp')"><NumInput v-model="form.speedTest.port" :min="1" :max="65535" /></FormItem>
                <FormItem :label="$t('tasks.edit.maxLoss')" flag="-tlr" :help="$t('tasks.edit.maxLossHelp')">
                  <NumInput v-model="form.speedTest.maxLossRate" :min="0" :max="1" :step="0.05" :decimals="2" />
                </FormItem>
                <FormItem :label="$t('tasks.edit.maxLatency')" flag="-tl" :help="$t('tasks.edit.maxLatencyHelp')"><NumInput v-model="form.speedTest.maxLatency" :min="0" :max="9999" suffix="ms" /></FormItem>
                <FormItem :label="$t('tasks.edit.minLatency')" flag="-tll" :help="$t('tasks.edit.minLatencyHelp')"><NumInput v-model="form.speedTest.minLatency" :min="0" :max="9999" suffix="ms" /></FormItem>
              </div>
            </section>

            <!-- 下载测速 -->
            <section class="grid grid-cols-1 gap-2">
              <h4 class="text-muted-foreground text-xs font-medium">{{ $t('tasks.edit.downloadTest') }}</h4>
              <div class="divide-y rounded-md border">
                <SettingRow :label="$t('tasks.edit.disableDownload')" flag="-dd" :description="$t('tasks.edit.disableDownloadDesc')">
                  <Switch v-model="form.speedTest.disableDownload" />
                </SettingRow>
              </div>
              <div v-if="!form.speedTest.disableDownload" class="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2 lg:grid-cols-4">
                <FormItem :label="$t('tasks.edit.downloadCount')" flag="-dn" :help="$t('tasks.edit.downloadCountHelp')"><NumInput v-model="form.speedTest.downloadCount" :min="1" :max="100" :suffix="$t('tasks.edit.ipUnit')" /></FormItem>
                <FormItem :label="$t('tasks.edit.downloadTime')" flag="-dt" :help="$t('tasks.edit.downloadTimeHelp')"><NumInput v-model="form.speedTest.downloadTime" :min="1" :max="120" :suffix="$t('tasks.edit.secondsUnit')" /></FormItem>
                <FormItem :label="$t('tasks.edit.minSpeed')" flag="-sl" :help="$t('tasks.edit.minSpeedHelp')">
                  <NumInput v-model="form.speedTest.minSpeed" :min="0" :step="0.5" :decimals="2" suffix="MB/s" />
                </FormItem>
                <p
                  v-if="form.speedTest.minSpeed > 0 && form.speedTest.maxLatency >= 9999"
                  class="text-warning flex items-start gap-1 self-end pb-2 text-xs sm:col-span-2 lg:col-span-1"
                >
                  <TriangleAlert class="mt-0.5 size-3.5 shrink-0" />{{ $t('tasks.edit.minSpeedWarn') }}
                </p>
              </div>
            </section>

            <!-- 测速地址与 HTTPing -->
            <section class="grid grid-cols-1 gap-2">
              <h4 class="text-muted-foreground text-xs font-medium">{{ $t('tasks.edit.urlSection') }}</h4>
              <FormItem :label="$t('tasks.edit.url')" flag="-url" :help="$t('tasks.edit.urlHelp')">
                <Input v-model="form.speedTest.url" :placeholder="$t('tasks.edit.urlPlaceholder')" class="font-mono" />
              </FormItem>
              <div class="divide-y rounded-md border">
                <SettingRow :label="$t('tasks.edit.httping')" flag="-httping" :description="$t('tasks.edit.httpingDesc')">
                  <Switch v-model="form.speedTest.httping" />
                </SettingRow>
              </div>
              <div v-if="form.speedTest.httping" class="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2 lg:grid-cols-4">
                <FormItem :label="$t('tasks.edit.httpingCode')" flag="-httping-code" :help="$t('tasks.edit.httpingCodeHelp')"><NumInput v-model="form.speedTest.httpingCode" :min="0" :max="599" /></FormItem>
                <FormItem :label="$t('tasks.edit.cfColo')" flag="-cfcolo" :help="$t('tasks.edit.cfColoHelp')" class="lg:col-span-3">
                  <Input v-model="form.speedTest.cfColo" :placeholder="$t('tasks.edit.cfColoPlaceholder')" class="font-mono" />
                </FormItem>
              </div>
            </section>

            <!-- IP 来源 -->
            <section class="grid grid-cols-1 gap-2">
              <h4 class="text-muted-foreground text-xs font-medium">{{ $t('tasks.edit.ipSource') }}</h4>
              <div class="divide-y rounded-md border">
                <SettingRow :label="$t('tasks.edit.allIP')" flag="-allip" :description="$t('tasks.edit.allIPDesc')">
                  <Switch v-model="form.speedTest.allIP" />
                </SettingRow>
                <SettingRow :label="$t('tasks.edit.ipRanges')" :description="form.speedTest.ipSource === 'custom' ? $t('tasks.edit.ipRangesCustomDesc') : $t('tasks.edit.ipRangesDefaultDesc')">
                  <Tabs v-model="form.speedTest.ipSource">
                    <TabsList class="h-8">
                      <TabsTrigger value="default" class="text-xs">{{ $t('tasks.edit.defaultFile') }}</TabsTrigger>
                      <TabsTrigger value="custom" class="text-xs">{{ $t('tasks.edit.custom') }}</TabsTrigger>
                    </TabsList>
                  </Tabs>
                </SettingRow>
              </div>
              <div v-if="form.speedTest.ipSource === 'custom'" :class="['grid grid-cols-1 gap-x-4 gap-y-3', showV4 && showV6 && 'md:grid-cols-2']">
                <FormItem v-if="showV4" :label="$t('tasks.edit.ipv4Ranges')" :help="$t('tasks.edit.rangesHelp')">
                  <Textarea v-model="form.speedTest.ipv4Ranges" class="text-code min-h-24 font-mono" placeholder="173.245.48.0/20&#10;104.16.0.0/13" />
                </FormItem>
                <FormItem v-if="showV6" :label="$t('tasks.edit.ipv6Ranges')" :help="$t('tasks.edit.rangesHelp')">
                  <Textarea v-model="form.speedTest.ipv6Ranges" class="text-code min-h-24 font-mono" placeholder="2606:4700::/32" />
                </FormItem>
              </div>
            </section>

            <Collapsible v-model:open="advancedOpen">
              <CollapsibleTrigger as-child>
                <button type="button" class="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs font-medium transition-colors">
                  <ChevronRight :class="['size-3.5 transition-transform duration-200', advancedOpen && 'rotate-90']" />{{ $t('tasks.edit.advanced') }}
                </button>
              </CollapsibleTrigger>
              <CollapsibleContent class="pt-2">
                <FormItem :label="$t('tasks.edit.extraArgs')" :help="$t('tasks.edit.extraArgsHelp')">
                  <Input v-model="form.speedTest.extraArgs" class="font-mono" :placeholder="$t('tasks.edit.extraArgsPlaceholder')" />
                </FormItem>
              </CollapsibleContent>
            </Collapsible>
          </CardContent>
        </Card>

        <!-- 目标记录 -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <Globe class="text-muted-foreground size-4" />{{ $t('tasks.targets') }}<ToneBadge>{{ form.targets.length }}</ToneBadge>
            </CardTitle>
            <CardDescription>{{ $t('tasks.edit.targetsDesc') }}</CardDescription>
            <CardAction>
              <Button type="button" variant="outline" size="sm" :disabled="!accounts.length" @click="addTarget"><Plus />{{ $t('common.add') }}</Button>
            </CardAction>
          </CardHeader>
          <CardContent>
            <EmptyState v-if="!accounts.length" compact :icon="UserRoundKey" :title="$t('tasks.edit.noAccount')" :description="$t('tasks.edit.noAccountDesc')">
              <Button type="button" size="sm" @click="router.push('/accounts')">{{ $t('tasks.edit.addAccount') }}</Button>
            </EmptyState>
            <EmptyState v-else-if="!form.targets.length" compact :icon="Globe" :title="$t('tasks.edit.v.targets')">
              <Button type="button" size="sm" variant="outline" @click="addTarget"><Plus />{{ $t('tasks.edit.addTarget') }}</Button>
            </EmptyState>
            <div v-else class="grid grid-cols-1 gap-2">
              <!-- md 以上的列标题 -->
              <div class="text-muted-foreground text-label hidden gap-2 px-0.5 font-medium md:grid md:grid-cols-[minmax(0,1.5fr)_minmax(0,1.3fr)_minmax(0,0.8fr)_84px_minmax(0,0.8fr)_32px]">
                <span>{{ $t('tasks.edit.colAccount') }}</span><span>{{ $t('tasks.edit.colDomain') }}</span><span>{{ $t('tasks.edit.colRr') }}</span><span>TTL</span><span>{{ $t('tasks.edit.colLineProxy') }}</span><span />
              </div>
              <div v-for="(t, i) in form.targets" :key="i" class="grid grid-cols-1 gap-1 border-b pb-2 last:border-b-0 last:pb-0">
                <div class="grid grid-cols-2 items-center gap-2 md:grid-cols-[minmax(0,1.5fr)_minmax(0,1.3fr)_minmax(0,0.8fr)_84px_minmax(0,0.8fr)_32px]">
                  <Select :model-value="t.accountId || undefined" @update:model-value="onAccountChange(t, $event)">
                    <!-- 触发器只显示服务商徽标 + 账号名，避免截断；下拉项仍显示“名称 · 服务商” -->
                    <SelectTrigger class="col-span-2 w-full md:col-span-1">
                      <span v-if="accountById(t.accountId)" class="flex min-w-0 items-center gap-2">
                        <BrandIcon
                          kind="provider"
                          :type="accountById(t.accountId)!.provider"
                          :name="meta.providerName(accountById(t.accountId)!.provider)"
                          :title="meta.providerName(accountById(t.accountId)!.provider)"
                          class="size-5 rounded"
                        />
                        <span class="truncate" :title="accountById(t.accountId)!.name">{{ accountById(t.accountId)!.name }}</span>
                      </span>
                      <SelectValue v-else :placeholder="$t('tasks.edit.pickAccount')" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem v-for="a in accounts" :key="a.id" :value="a.id">
                        <BrandIcon kind="provider" :type="a.provider" :name="meta.providerName(a.provider)" class="size-5 rounded" />
                        {{ a.name }} <span class="text-muted-foreground text-xs">· {{ meta.providerName(a.provider) }}</span>
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <SuggestInput
                    v-model="t.domain"
                    class="col-span-2 md:col-span-1"
                    :options="domains[t.accountId]?.list ?? []"
                    :loading="domains[t.accountId]?.loading"
                    placeholder="example.com"
                    @open="t.accountId && loadDomains(t.accountId)"
                  />
                  <Input v-model="t.rr" placeholder="www / @" class="font-mono" :title="$t('tasks.edit.colRr')" />
                  <NumInput v-model="t.ttl" :min="1" :max="86400" :title="isCloudflare(t.accountId) ? $t('tasks.edit.ttlAutoTitle') : 'TTL'" />
                  <Input v-if="t.accountId && !isCloudflare(t.accountId)" v-model="t.line" :placeholder="$t('tasks.edit.defaultLine')" />
                  <label v-else class="flex h-9 cursor-pointer items-center gap-2 text-sm">
                    <Switch v-model="t.proxied" />{{ $t('tasks.edit.proxied') }}
                  </label>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    class="text-muted-foreground hover:text-destructive size-8 justify-self-end"
                    :title="$t('common.delete')"
                    @click="form.targets.splice(i, 1)"
                  >
                    <Trash2 />
                  </Button>
                </div>
                <div class="flex flex-wrap items-center gap-x-3 gap-y-1 px-0.5 text-xs">
                  <span :class="['font-mono break-all', fqdn(t) ? 'text-primary' : 'text-muted-foreground']">{{ fqdn(t) || $t('tasks.edit.noDomain') }}</span>
                  <span v-if="isCloudflare(t.accountId)" class="text-muted-foreground">{{ $t('tasks.edit.ttlAuto') }}</span>
                  <button type="button" class="text-muted-foreground hover:text-foreground inline-flex items-center gap-1" @click="viewRecords(t)">
                    <Search class="size-3" />{{ $t('tasks.edit.viewRecords') }}
                  </button>
                  <span v-if="isCloudflare(t.accountId) && t.proxied" class="text-warning inline-flex items-center gap-1">
                    <TriangleAlert class="size-3.5" />{{ $t('tasks.edit.proxiedWarn') }}
                  </span>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- 更新策略 + 通知 -->
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle class="flex items-center gap-2"><SlidersHorizontal class="text-muted-foreground size-4" />{{ $t('tasks.edit.updatePolicy') }}</CardTitle>
            </CardHeader>
            <CardContent class="grid grid-cols-1 gap-3">
              <FormItem :label="$t('tasks.edit.recordCount')" :help="$t('tasks.edit.recordCountHelp')">
                <NumInput v-model="form.update.recordCount" :min="1" :max="10" :suffix="$t('tasks.edit.recordUnit')" class="max-w-40" />
              </FormItem>
              <div class="divide-y rounded-md border">
                <SettingRow :label="$t('tasks.edit.skipUnchanged')" :description="$t('tasks.edit.skipUnchangedDesc')">
                  <Switch v-model="form.update.skipUnchanged" />
                </SettingRow>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle class="flex items-center gap-2"><Bell class="text-muted-foreground size-4" />{{ $t('tasks.edit.notify') }}</CardTitle>
              <CardDescription>
                {{ $t('tasks.edit.notifyDesc') }} · <router-link to="/notifiers" class="text-primary hover:underline">{{ $t('tasks.edit.manage') }}</router-link>
              </CardDescription>
            </CardHeader>
            <CardContent>
              <p v-if="!notifiers.length" class="text-muted-foreground text-xs">{{ $t('tasks.edit.noNotifiers') }}</p>
              <div v-else class="divide-y rounded-md border">
                <label v-for="nt in notifiers" :key="nt.id" class="hover:bg-accent/40 flex cursor-pointer items-center gap-3 px-3 py-2">
                  <Checkbox :model-value="form.notifierIds.includes(nt.id)" @update:model-value="toggleNotifier(nt.id, !!$event)" />
                  <BrandIcon kind="notifier" :type="nt.type" :name="meta.notifierName(nt.type)" class="size-6" />
                  <span class="min-w-0 flex-1 truncate text-sm" :title="nt.name">{{ nt.name }}</span>
                  <span class="text-muted-foreground text-xs">{{ meta.notifierName(nt.type) }}{{ nt.enabled ? '' : ` · ${$t('common.disabled')}` }}</span>
                </label>
              </div>
            </CardContent>
          </Card>
        </div>

        <!-- 吸底保存栏（仅在内容区宽度内） -->
        <div class="bg-background/85 sticky bottom-0 z-10 flex justify-end gap-2 border-t py-3 backdrop-blur">
          <Button type="button" variant="outline" @click="router.push('/tasks')">{{ $t('common.cancel') }}</Button>
          <Button type="submit" :disabled="saving"><Loader2 v-if="saving" class="animate-spin" /><Save v-else />{{ $t('tasks.edit.save') }}</Button>
        </div>
      </form>

      <!-- 右侧摘要（xl 以上） -->
      <aside class="hidden xl:sticky xl:top-18 xl:block">
        <Card>
          <CardHeader>
            <CardTitle>{{ $t('tasks.edit.summary') }}</CardTitle>
          </CardHeader>
          <CardContent class="grid grid-cols-1 gap-3 text-sm">
            <div>
              <div class="text-muted-foreground text-xs">{{ $t('common.status') }}</div>
              <div class="mt-0.5 flex items-center gap-2">
                <ToneBadge :tone="form.enabled ? 'success' : 'neutral'">{{ form.enabled ? $t('common.enabled') : $t('common.disabled') }}</ToneBadge>
                <ToneBadge>{{ ipTypeLabel[form.ipType] }}</ToneBadge>
              </div>
            </div>
            <div>
              <div class="text-muted-foreground text-xs">{{ $t('tasks.schedule') }}</div>
              <div class="mt-0.5">{{ describeCron(form.cron) }}</div>
              <div v-if="form.cron.trim() && nextRuns.length" class="text-muted-foreground text-xs tabular-nums">
                {{ $t('tasks.edit.nextRun', { time: fmtTime(nextRuns[0], 'MM-DD HH:mm'), rel: fromNow(nextRuns[0]) }) }}
              </div>
            </div>
            <div>
              <div class="text-muted-foreground text-xs">{{ $t('tasks.edit.perType') }}</div>
              <div class="mt-0.5 tabular-nums">{{ $t('tasks.edit.topN', { n: form.update.recordCount || 1 }) }}{{ form.update.skipUnchanged ? ` · ${$t('tasks.edit.skipShort')}` : '' }}</div>
            </div>
            <div>
              <div class="text-muted-foreground text-xs">{{ $t('tasks.edit.willWrite', { n: summaryRecords.length }) }}</div>
              <ul v-if="summaryRecords.length" class="mt-1 grid grid-cols-1 gap-1">
                <li v-for="r in summaryRecords" :key="r.key" class="flex min-w-0 items-center gap-1.5">
                  <ToneBadge class="shrink-0 px-1">{{ r.type }}</ToneBadge>
                  <span class="text-code min-w-0 truncate font-mono" :title="r.fqdn">{{ r.fqdn }}</span>
                </li>
              </ul>
              <p v-else class="text-muted-foreground mt-0.5 text-xs">{{ $t('tasks.edit.noTargetsYet') }}</p>
            </div>
            <div>
              <div class="text-muted-foreground text-xs">{{ $t('tasks.edit.notifiers') }}</div>
              <div class="mt-0.5">{{ form.notifierIds.length ? $t('tasks.edit.notifierCount', { n: form.notifierIds.length }) : $t('tasks.edit.noNotify') }}</div>
            </div>
          </CardContent>
        </Card>
      </aside>
    </div>

    <Dialog v-model:open="rec.open">
      <DialogContent class="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>{{ $t('tasks.edit.existingRecords') }}</DialogTitle>
          <DialogDescription class="font-mono">{{ rec.title }}</DialogDescription>
        </DialogHeader>
        <div class="max-h-[60vh] overflow-auto rounded-md border">
          <div v-if="rec.loading" class="grid grid-cols-1 gap-2 p-4"><Skeleton v-for="i in 3" :key="i" class="h-8" /></div>
          <EmptyState v-else-if="rec.error" :icon="CircleX" compact :title="$t('tasks.edit.queryFailed')" />
          <EmptyState v-else-if="!rec.records.length" :icon="Globe" compact :title="$t('tasks.edit.noRecords')" :description="$t('tasks.edit.noRecordsDesc')" />
          <Table v-else>
            <TableHeader>
              <TableRow>
                <TableHead>{{ $t('tasks.edit.colFqdn') }}</TableHead>
                <TableHead>{{ $t('common.type') }}</TableHead>
                <TableHead>{{ $t('tasks.edit.colValue') }}</TableHead>
                <TableHead class="text-right">TTL</TableHead>
                <TableHead>{{ $t('tasks.edit.colProxyLine') }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="r in rec.records" :key="r.id">
                <TableCell class="font-medium">{{ r.fqdn }}</TableCell>
                <TableCell><ToneBadge>{{ r.type }}</ToneBadge></TableCell>
                <TableCell class="text-code font-mono">{{ r.value }}</TableCell>
                <TableCell class="text-right">{{ r.ttl }}</TableCell>
                <TableCell>
                  <ToneBadge v-if="r.proxied" tone="warning">{{ $t('tasks.edit.isProxied') }}</ToneBadge>
                  <span v-else class="text-muted-foreground text-xs">{{ r.line || '-' }}</span>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>
