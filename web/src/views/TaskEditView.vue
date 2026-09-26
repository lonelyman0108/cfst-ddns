<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
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
  return confirm({ title: '放弃未保存的修改？', description: '离开后本页的修改将丢失。', confirmText: '离开', destructive: true })
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

async function viewRecords(t: Target) {
  if (!t.accountId || !t.domain.trim()) {
    toast.warning('请先选择账号并填写主域名')
    return
  }
  Object.assign(rec, { open: true, loading: true, records: [], error: false, title: fqdn(t) || t.domain })
  try {
    rec.records = (await accountsApi.records(t.accountId, t.domain.trim(), t.rr.trim() || undefined)) ?? []
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

const TIPS = {
  threads: '-n 延迟测速线程数。越多延迟测速越快，性能弱的设备（如路由器）请勿设置过高。默认 200，最多 1000。',
  pingTimes: '-t 延迟测速次数。单个 IP 延迟测速的次数。默认 4 次。',
  downloadCount: '-dn 下载测速数量。延迟测速并排序后，从最低延迟起进行下载测速的数量。默认 10 个。',
  downloadTime: '-dt 下载测速时间。单个 IP 下载测速的最长时间（秒），不宜太短。默认 10 秒。',
  port: '-tp 测速端口。延迟测速 / 下载测速时使用的端口。默认 443。',
  url: '-url 测速地址。延迟测速（HTTPing）/ 下载测速时使用的地址。留空使用 cfst 内置地址，内置地址不保证可用，建议自建。',
  httping: '-httping 切换测速模式。开启后延迟测速改用 HTTP 协议（使用测速地址），默认 TCPing。',
  httpingCode: '-httping-code 有效状态码。HTTPing 延迟测速时网页返回的有效 HTTP 状态码，仅限一个。0 表示使用默认（200、301、302）。',
  cfColo: '-cfcolo 匹配指定地区。地区码为当地机场三字码，英文逗号分隔，如 HKG,KHH,NRT,LAX。仅 HTTPing 模式可用。',
  maxLatency: '-tl 平均延迟上限（ms）。只输出低于该平均延迟的 IP。默认 9999。',
  minLatency: '-tll 平均延迟下限（ms）。只输出高于该平均延迟的 IP。默认 0。',
  maxLossRate: '-tlr 丢包率上限。只输出低于或等于该丢包率的 IP，范围 0.00~1.00，0 表示过滤掉任何丢包的 IP。默认 1。',
  minSpeed: '-sl 下载速度下限（MB/s）。只输出高于该速度的 IP，凑够下载测速数量才会停止。\n建议搭配延迟上限 -tl 使用，避免因凑不够数量而一直测速。默认 0。',
  disableDownload: '-dd 禁用下载测速。禁用后测速结果按延迟排序（默认按下载速度排序）。',
  allIP: '-allip 测速全部 IP。对 IP 段中的每个 IP 进行测速（仅支持 IPv4），耗时显著增加。默认每个 /24 段随机测速一个 IP。',
  ipSource: '默认：使用 cfst 目录下的 ip.txt / ipv6.txt（可在「cfst 管理」中编辑）。\n自定义：为本任务单独指定 IP 段。',
  extraArgs: '附加到 cfst 命令行的原始参数，用空格分隔。仅在明确了解参数含义时使用，可能与上方配置冲突。',
  recordCount: '每种记录类型（A / AAAA）写入测速结果中排名前 N 的 IP。N > 1 时会创建多条同名记录，实现 DNS 轮询负载均衡。',
  skipUnchanged: '记录值与本次选出的 IP 一致时不调用服务商更新接口，减少 API 调用。',
}

// ---------- 保存 ----------
function validate(): string {
  if (!form.name.trim()) return '请填写任务名称'
  if (!form.targets.length) return '请至少添加一个目标记录'
  for (const [i, t] of form.targets.entries()) {
    const n = `目标记录 #${i + 1}`
    if (!t.accountId) return `${n}：请选择 DNS 账号`
    if (!t.domain.trim()) return `${n}：请填写主域名`
    if (!t.rr.trim()) return `${n}：请填写主机记录（根域名填 @）`
  }
  if (form.update.recordCount < 1 || form.update.recordCount > 10) return '写入 IP 数量需在 1–10 之间'
  if (form.speedTest.ipSource === 'custom') {
    if (showV4.value && !form.speedTest.ipv4Ranges.trim()) return '请填写自定义 IPv4 段'
    if (showV6.value && !form.speedTest.ipv6Ranges.trim()) return '请填写自定义 IPv6 段'
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
    toast.success('任务已保存')
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
        <h1 class="page-title truncate">{{ isNew ? '新建任务' : form.name || '编辑任务' }}</h1>
        <p class="text-muted-foreground mt-0.5">配置测速参数、写入策略与目标 DNS 记录</p>
      </div>
      <label v-if="!loading && !loadFailed" class="flex cursor-pointer items-center gap-2 text-sm font-medium">
        <Switch v-model="form.enabled" />{{ form.enabled ? '已启用' : '已停用' }}
      </label>
    </div>

    <div v-if="loading" class="grid max-w-5xl gap-4">
      <Skeleton v-for="i in 4" :key="i" class="h-40 rounded-xl" />
    </div>

    <EmptyState v-else-if="loadFailed" :icon="CircleX" title="加载失败" description="无法获取任务数据">
      <Button variant="outline" size="sm" @click="load">重试</Button>
    </EmptyState>

    <div v-else class="grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_280px]">
      <form class="grid min-w-0 gap-4" @submit.prevent="save">
        <!-- 基本信息 -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2"><Settings2 class="text-muted-foreground size-4" />基本信息</CardTitle>
          </CardHeader>
          <CardContent class="grid gap-x-4 gap-y-3 md:grid-cols-3">
            <FormItem label="任务名称" required for="name" class="md:col-span-2">
              <Input id="name" v-model="form.name" maxlength="64" placeholder="如：主站优选" />
            </FormItem>
            <FormItem label="IP 类型" tip="IPv4 写入 A 记录，IPv6 写入 AAAA 记录；双栈会分别测速。">
              <Tabs :model-value="form.ipType" @update:model-value="setIpType">
                <TabsList class="h-9 w-full">
                  <TabsTrigger value="v4">IPv4</TabsTrigger>
                  <TabsTrigger value="v6">IPv6</TabsTrigger>
                  <TabsTrigger value="both">双栈</TabsTrigger>
                </TabsList>
              </Tabs>
            </FormItem>
            <FormItem label="执行周期" class="md:col-span-3">
              <CronInput v-model="form.cron" @next="nextRuns = $event" />
            </FormItem>
          </CardContent>
        </Card>

        <!-- 测速参数 -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2"><Gauge class="text-muted-foreground size-4" />测速参数</CardTitle>
            <CardDescription>对应 CloudflareSpeedTest 命令行参数，悬停问号查看说明</CardDescription>
          </CardHeader>
          <CardContent class="grid gap-4">
            <!-- 延迟测速 -->
            <section class="grid gap-2">
              <h4 class="text-muted-foreground text-xs font-medium">延迟测速</h4>
              <div class="grid gap-x-4 gap-y-3 sm:grid-cols-2 lg:grid-cols-4">
                <FormItem label="线程数" :tip="TIPS.threads"><NumInput v-model="form.speedTest.threads" :min="1" :max="1000" /></FormItem>
                <FormItem label="测速次数" :tip="TIPS.pingTimes"><NumInput v-model="form.speedTest.pingTimes" :min="1" :max="100" suffix="次" /></FormItem>
                <FormItem label="端口" :tip="TIPS.port"><NumInput v-model="form.speedTest.port" :min="1" :max="65535" /></FormItem>
                <FormItem label="丢包率上限" :tip="TIPS.maxLossRate">
                  <NumInput v-model="form.speedTest.maxLossRate" :min="0" :max="1" :step="0.05" :decimals="2" />
                </FormItem>
                <FormItem label="延迟上限" :tip="TIPS.maxLatency"><NumInput v-model="form.speedTest.maxLatency" :min="0" :max="9999" suffix="ms" /></FormItem>
                <FormItem label="延迟下限" :tip="TIPS.minLatency"><NumInput v-model="form.speedTest.minLatency" :min="0" :max="9999" suffix="ms" /></FormItem>
              </div>
            </section>

            <!-- 下载测速 -->
            <section class="grid gap-2">
              <h4 class="text-muted-foreground text-xs font-medium">下载测速</h4>
              <div class="divide-y rounded-md border">
                <SettingRow label="禁用下载测速" description="禁用后按延迟排序，测速更快" :tip="TIPS.disableDownload">
                  <Switch v-model="form.speedTest.disableDownload" />
                </SettingRow>
              </div>
              <div v-if="!form.speedTest.disableDownload" class="grid gap-x-4 gap-y-3 sm:grid-cols-2 lg:grid-cols-4">
                <FormItem label="测速数量" :tip="TIPS.downloadCount"><NumInput v-model="form.speedTest.downloadCount" :min="1" :max="100" suffix="个" /></FormItem>
                <FormItem label="测速时间" :tip="TIPS.downloadTime"><NumInput v-model="form.speedTest.downloadTime" :min="1" :max="120" suffix="秒" /></FormItem>
                <FormItem label="速度下限" :tip="TIPS.minSpeed">
                  <NumInput v-model="form.speedTest.minSpeed" :min="0" :step="0.5" :decimals="2" suffix="MB/s" />
                </FormItem>
                <p
                  v-if="form.speedTest.minSpeed > 0 && form.speedTest.maxLatency >= 9999"
                  class="text-warning flex items-start gap-1 self-end pb-2 text-xs sm:col-span-2 lg:col-span-1"
                >
                  <TriangleAlert class="mt-0.5 size-3.5 shrink-0" />建议同时设置延迟上限，避免因凑不够数量而长时间测速
                </p>
              </div>
            </section>

            <!-- 测速地址与 HTTPing -->
            <section class="grid gap-2">
              <h4 class="text-muted-foreground text-xs font-medium">测速地址与 HTTPing</h4>
              <FormItem :tip="TIPS.url" label="测速地址">
                <Input v-model="form.speedTest.url" placeholder="留空使用 cfst 内置地址" class="font-mono" />
              </FormItem>
              <div class="divide-y rounded-md border">
                <SettingRow label="HTTPing 模式" description="延迟测速改用 HTTP 协议（默认 TCPing）" :tip="TIPS.httping">
                  <Switch v-model="form.speedTest.httping" />
                </SettingRow>
              </div>
              <div v-if="form.speedTest.httping" class="grid gap-x-4 gap-y-3 sm:grid-cols-2 lg:grid-cols-4">
                <FormItem label="有效状态码" :tip="TIPS.httpingCode"><NumInput v-model="form.speedTest.httpingCode" :min="0" :max="599" /></FormItem>
                <FormItem label="匹配地区" :tip="TIPS.cfColo" class="lg:col-span-3">
                  <Input v-model="form.speedTest.cfColo" placeholder="HKG,NRT,LAX（留空为所有地区）" class="font-mono" />
                </FormItem>
              </div>
            </section>

            <!-- IP 来源 -->
            <section class="grid gap-2">
              <h4 class="text-muted-foreground text-xs font-medium">IP 来源</h4>
              <div class="divide-y rounded-md border">
                <SettingRow label="测速全部 IP" description="对每个 IP 测速（仅 IPv4），耗时显著增加" :tip="TIPS.allIP">
                  <Switch v-model="form.speedTest.allIP" />
                </SettingRow>
                <SettingRow label="IP 段" :description="form.speedTest.ipSource === 'custom' ? '为本任务单独指定 IP 段' : '使用 cfst 目录下的 ip.txt / ipv6.txt'" :tip="TIPS.ipSource">
                  <Tabs v-model="form.speedTest.ipSource">
                    <TabsList class="h-8">
                      <TabsTrigger value="default" class="text-xs">默认文件</TabsTrigger>
                      <TabsTrigger value="custom" class="text-xs">自定义</TabsTrigger>
                    </TabsList>
                  </Tabs>
                </SettingRow>
              </div>
              <div v-if="form.speedTest.ipSource === 'custom'" :class="['grid gap-x-4 gap-y-3', showV4 && showV6 && 'md:grid-cols-2']">
                <FormItem v-if="showV4" label="IPv4 段" help="每行一个或以逗号分隔">
                  <Textarea v-model="form.speedTest.ipv4Ranges" class="text-code min-h-24 font-mono" placeholder="173.245.48.0/20&#10;104.16.0.0/13" />
                </FormItem>
                <FormItem v-if="showV6" label="IPv6 段" help="每行一个或以逗号分隔">
                  <Textarea v-model="form.speedTest.ipv6Ranges" class="text-code min-h-24 font-mono" placeholder="2606:4700::/32" />
                </FormItem>
              </div>
            </section>

            <Collapsible v-model:open="advancedOpen">
              <CollapsibleTrigger as-child>
                <button type="button" class="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs font-medium transition-colors">
                  <ChevronRight :class="['size-3.5 transition-transform duration-200', advancedOpen && 'rotate-90']" />高级参数
                </button>
              </CollapsibleTrigger>
              <CollapsibleContent class="pt-2">
                <FormItem label="附加参数" :tip="TIPS.extraArgs">
                  <Input v-model="form.speedTest.extraArgs" class="font-mono" placeholder="如 -tlr 0.2" />
                </FormItem>
              </CollapsibleContent>
            </Collapsible>
          </CardContent>
        </Card>

        <!-- 目标记录 -->
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <Globe class="text-muted-foreground size-4" />目标记录<ToneBadge>{{ form.targets.length }}</ToneBadge>
            </CardTitle>
            <CardDescription>测速结果写入以下 DNS 记录，可跨账号、跨域名</CardDescription>
            <CardAction>
              <Button type="button" variant="outline" size="sm" :disabled="!accounts.length" @click="addTarget"><Plus />添加</Button>
            </CardAction>
          </CardHeader>
          <CardContent>
            <EmptyState v-if="!accounts.length" compact :icon="UserRoundKey" title="还没有 DNS 账号" description="添加服务商账号后才能配置目标记录">
              <Button type="button" size="sm" @click="router.push('/accounts')">添加 DNS 账号</Button>
            </EmptyState>
            <EmptyState v-else-if="!form.targets.length" compact :icon="Globe" title="至少添加一个目标记录">
              <Button type="button" size="sm" variant="outline" @click="addTarget"><Plus />添加目标记录</Button>
            </EmptyState>
            <div v-else class="grid gap-2">
              <!-- md 以上的列标题 -->
              <div class="text-muted-foreground text-label hidden gap-2 px-0.5 font-medium md:grid md:grid-cols-[minmax(0,1.5fr)_minmax(0,1.3fr)_minmax(0,0.8fr)_84px_minmax(0,0.8fr)_32px]">
                <span>DNS 账号</span><span>主域名</span><span>主机记录</span><span>TTL</span><span>线路 / 代理</span><span />
              </div>
              <div v-for="(t, i) in form.targets" :key="i" class="grid gap-1 border-b pb-2 last:border-b-0 last:pb-0">
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
                        <span class="truncate">{{ accountById(t.accountId)!.name }}</span>
                      </span>
                      <SelectValue v-else placeholder="选择账号" />
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
                  <Input v-model="t.rr" placeholder="www / @" class="font-mono" title="主机记录" />
                  <NumInput v-model="t.ttl" :min="1" :max="86400" :title="isCloudflare(t.accountId) ? 'TTL（1 = 自动）' : 'TTL'" />
                  <Input v-if="t.accountId && !isCloudflare(t.accountId)" v-model="t.line" placeholder="默认线路" />
                  <label v-else class="flex h-9 cursor-pointer items-center gap-2 text-sm">
                    <Switch v-model="t.proxied" />代理
                  </label>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    class="text-muted-foreground hover:text-destructive size-8 justify-self-end"
                    title="删除"
                    @click="form.targets.splice(i, 1)"
                  >
                    <Trash2 />
                  </Button>
                </div>
                <div class="flex flex-wrap items-center gap-x-3 gap-y-1 px-0.5 text-xs">
                  <span :class="['font-mono break-all', fqdn(t) ? 'text-primary' : 'text-muted-foreground']">{{ fqdn(t) || '未填写域名' }}</span>
                  <span v-if="isCloudflare(t.accountId)" class="text-muted-foreground">TTL 1 = 自动</span>
                  <button type="button" class="text-muted-foreground hover:text-foreground inline-flex items-center gap-1" @click="viewRecords(t)">
                    <Search class="size-3" />查看现有记录
                  </button>
                  <span v-if="isCloudflare(t.accountId) && t.proxied" class="text-warning inline-flex items-center gap-1">
                    <TriangleAlert class="size-3.5" />开启代理后访问者解析到 Cloudflare 分配的 IP，优选 IP 将不生效
                  </span>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- 更新策略 + 通知 -->
        <div class="grid gap-4 md:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle class="flex items-center gap-2"><SlidersHorizontal class="text-muted-foreground size-4" />更新策略</CardTitle>
            </CardHeader>
            <CardContent class="grid gap-3">
              <FormItem label="写入 IP 数量" :tip="TIPS.recordCount" help="多条同名记录实现负载均衡（1–10）">
                <NumInput v-model="form.update.recordCount" :min="1" :max="10" suffix="条" class="max-w-40" />
              </FormItem>
              <div class="divide-y rounded-md border">
                <SettingRow label="IP 未变化时跳过" description="记录值未变时不调用更新接口" :tip="TIPS.skipUnchanged">
                  <Switch v-model="form.update.skipUnchanged" />
                </SettingRow>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle class="flex items-center gap-2"><Bell class="text-muted-foreground size-4" />通知</CardTitle>
              <CardDescription>
                触发条件在各渠道中设置 · <router-link to="/notifiers" class="text-primary hover:underline">管理</router-link>
              </CardDescription>
            </CardHeader>
            <CardContent>
              <p v-if="!notifiers.length" class="text-muted-foreground text-xs">暂无通知渠道。</p>
              <div v-else class="divide-y rounded-md border">
                <label v-for="nt in notifiers" :key="nt.id" class="hover:bg-accent/40 flex cursor-pointer items-center gap-3 px-3 py-2">
                  <Checkbox :model-value="form.notifierIds.includes(nt.id)" @update:model-value="toggleNotifier(nt.id, !!$event)" />
                  <BrandIcon kind="notifier" :type="nt.type" :name="meta.notifierName(nt.type)" class="size-6" />
                  <span class="min-w-0 flex-1 truncate text-sm">{{ nt.name }}</span>
                  <span class="text-muted-foreground text-xs">{{ meta.notifierName(nt.type) }}{{ nt.enabled ? '' : ' · 已停用' }}</span>
                </label>
              </div>
            </CardContent>
          </Card>
        </div>

        <!-- 吸底保存栏（仅在内容区宽度内） -->
        <div class="bg-background/85 sticky bottom-0 z-10 flex justify-end gap-2 border-t py-3 backdrop-blur">
          <Button type="button" variant="outline" @click="router.push('/tasks')">取消</Button>
          <Button type="submit" :disabled="saving"><Loader2 v-if="saving" class="animate-spin" /><Save v-else />保存任务</Button>
        </div>
      </form>

      <!-- 右侧摘要（xl 以上） -->
      <aside class="hidden xl:sticky xl:top-18 xl:block">
        <Card>
          <CardHeader>
            <CardTitle>摘要</CardTitle>
          </CardHeader>
          <CardContent class="grid gap-3 text-sm">
            <div>
              <div class="text-muted-foreground text-xs">状态</div>
              <div class="mt-0.5 flex items-center gap-2">
                <ToneBadge :tone="form.enabled ? 'success' : 'neutral'">{{ form.enabled ? '已启用' : '已停用' }}</ToneBadge>
                <ToneBadge>{{ ipTypeLabel[form.ipType] }}</ToneBadge>
              </div>
            </div>
            <div>
              <div class="text-muted-foreground text-xs">执行周期</div>
              <div class="mt-0.5">{{ describeCron(form.cron) }}</div>
              <div v-if="form.cron.trim() && nextRuns.length" class="text-muted-foreground text-xs tabular-nums">
                下次 {{ fmtTime(nextRuns[0], 'MM-DD HH:mm') }}（{{ fromNow(nextRuns[0]) }}）
              </div>
            </div>
            <div>
              <div class="text-muted-foreground text-xs">每种类型写入</div>
              <div class="mt-0.5 tabular-nums">前 {{ form.update.recordCount || 1 }} 个 IP{{ form.update.skipUnchanged ? ' · 未变化跳过' : '' }}</div>
            </div>
            <div>
              <div class="text-muted-foreground text-xs">将写入的记录（{{ summaryRecords.length }}）</div>
              <ul v-if="summaryRecords.length" class="mt-1 grid gap-1">
                <li v-for="r in summaryRecords" :key="r.key" class="flex items-center gap-1.5">
                  <ToneBadge class="px-1">{{ r.type }}</ToneBadge>
                  <span class="text-code truncate font-mono">{{ r.fqdn }}</span>
                </li>
              </ul>
              <p v-else class="text-muted-foreground mt-0.5 text-xs">尚未填写目标记录</p>
            </div>
            <div>
              <div class="text-muted-foreground text-xs">通知渠道</div>
              <div class="mt-0.5">{{ form.notifierIds.length ? `${form.notifierIds.length} 个` : '不通知' }}</div>
            </div>
          </CardContent>
        </Card>
      </aside>
    </div>

    <Dialog v-model:open="rec.open">
      <DialogContent class="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>现有记录</DialogTitle>
          <DialogDescription class="font-mono">{{ rec.title }}</DialogDescription>
        </DialogHeader>
        <div class="max-h-[60vh] overflow-auto rounded-md border">
          <div v-if="rec.loading" class="grid gap-2 p-4"><Skeleton v-for="i in 3" :key="i" class="h-8" /></div>
          <EmptyState v-else-if="rec.error" :icon="CircleX" compact title="查询失败" />
          <EmptyState v-else-if="!rec.records.length" :icon="Globe" compact title="没有匹配的记录" description="保存并执行任务后会自动创建" />
          <Table v-else>
            <TableHeader>
              <TableRow>
                <TableHead>域名</TableHead>
                <TableHead>类型</TableHead>
                <TableHead>值</TableHead>
                <TableHead class="text-right">TTL</TableHead>
                <TableHead>代理 / 线路</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="r in rec.records" :key="r.id">
                <TableCell class="font-medium">{{ r.fqdn }}</TableCell>
                <TableCell><ToneBadge>{{ r.type }}</ToneBadge></TableCell>
                <TableCell class="text-code font-mono">{{ r.value }}</TableCell>
                <TableCell class="text-right">{{ r.ttl }}</TableCell>
                <TableCell>
                  <ToneBadge v-if="r.proxied" tone="warning">已代理</ToneBadge>
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
