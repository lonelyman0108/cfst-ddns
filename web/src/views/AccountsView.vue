<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { toast } from 'vue-sonner'
import {
  ArrowLeft,
  CircleCheck,
  CircleX,
  EllipsisVertical,
  ExternalLink,
  Loader2,
  Pencil,
  PlugZap,
  Plus,
  RefreshCw,
  Trash2,
  UserRoundKey,
} from '@lucide/vue'
import { accountsApi } from '@/api'
import type { Account, Config, TestResult, TypeMeta } from '@/api/types'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import EmptyState from '@/components/EmptyState.vue'
import FormItem from '@/components/FormItem.vue'
import PageHeader from '@/components/PageHeader.vue'
import SchemaForm, { mergeSchemaDefaults, schemaDefaults } from '@/components/SchemaForm.vue'
import BrandIcon from '@/components/BrandIcon.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import TypePicker from '@/components/TypePicker.vue'
import { confirm } from '@/composables/useConfirm'
import { useMetaStore } from '@/stores/meta'
import { fromNow } from '@/utils/format'

const meta = useMetaStore()
const list = ref<Account[]>([])
const loading = ref(false)
const loaded = ref(false)
const testing = ref<Record<number, boolean>>({})

async function load() {
  loading.value = true
  try {
    const [accs] = await Promise.all([accountsApi.list(), meta.loadProviders().catch(() => {})])
    list.value = accs ?? []
    loaded.value = true
  } catch {
    /* 已提示 */
  } finally {
    loading.value = false
  }
}

onMounted(load)

// ---------- 对话框 ----------
const dlg = reactive({
  open: false,
  step: 1 as 1 | 2,
  id: 0,
  provider: '',
  name: '',
  remark: '',
  nameError: '',
  config: {} as Config,
  saving: false,
  testing: false,
  result: null as TestResult | null,
  key: 0,
})
const schemaRef = ref<InstanceType<typeof SchemaForm>>()

const providerMeta = computed<TypeMeta | undefined>(() => meta.providers.find((p) => p.type === dlg.provider))
const editing = computed(() => dlg.id > 0)

function openCreate() {
  Object.assign(dlg, { open: true, step: 1, id: 0, provider: '', name: '', remark: '', nameError: '', config: {}, result: null })
  dlg.key++
}

function pickProvider(p: TypeMeta) {
  dlg.provider = p.type
  dlg.config = schemaDefaults(p.fields)
  if (!dlg.name) dlg.name = p.name
  dlg.step = 2
  dlg.result = null
  dlg.key++
}

function openEdit(a: Account) {
  const pm = meta.providers.find((p) => p.type === a.provider)
  Object.assign(dlg, {
    open: true,
    step: 2,
    id: a.id,
    provider: a.provider,
    name: a.name,
    remark: a.remark,
    nameError: '',
    config: pm ? mergeSchemaDefaults(pm.fields, a.config) : { ...(a.config ?? {}) },
    result: null,
  })
  dlg.key++
}

async function test() {
  if (!schemaRef.value?.validate()) return
  dlg.testing = true
  dlg.result = null
  try {
    // 编辑时带上 id：未修改的密钥（******）由后端用已保存值补齐，其余字段按表单测试
    dlg.result = await accountsApi.test({
      provider: dlg.provider,
      config: { ...dlg.config },
      ...(editing.value ? { id: dlg.id } : {}),
    })
  } catch {
    /* 已提示 */
  } finally {
    dlg.testing = false
  }
}

async function save() {
  dlg.nameError = dlg.name.trim() ? '' : '请填写名称'
  const ok = schemaRef.value?.validate() ?? false
  if (dlg.nameError || !ok) return
  dlg.saving = true
  try {
    if (editing.value) {
      await accountsApi.update(dlg.id, { name: dlg.name.trim(), config: { ...dlg.config }, remark: dlg.remark })
    } else {
      await accountsApi.create({ name: dlg.name.trim(), provider: dlg.provider, config: { ...dlg.config }, remark: dlg.remark })
    }
    toast.success('账号已保存')
    dlg.open = false
    load()
  } catch {
    /* 已提示 */
  } finally {
    dlg.saving = false
  }
}

async function testSaved(a: Account) {
  testing.value[a.id] = true
  try {
    const r = await accountsApi.testSaved(a.id)
    if (r.ok) toast.success(`「${a.name}」连接成功`, { description: r.message || undefined })
    else toast.error(`「${a.name}」连接失败`, { description: r.message || undefined })
  } catch {
    /* 已提示 */
  } finally {
    testing.value[a.id] = false
  }
}

async function remove(a: Account) {
  const ok = await confirm({
    title: `删除 DNS 账号「${a.name}」？`,
    description: a.taskCount ? `该账号被 ${a.taskCount} 个任务引用，需先从任务中移除。` : '删除后无法恢复。',
    confirmText: '删除',
    destructive: true,
  })
  if (!ok) return
  try {
    await accountsApi.remove(a.id)
    toast.success('账号已删除')
    load()
  } catch {
    /* 409 等错误已由拦截器显示后端提示 */
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="DNS 账号" description="DNS 服务商凭据，加密存储">
      <Button variant="outline" size="sm" :disabled="loading" @click="load"><RefreshCw :class="loading ? 'animate-spin' : ''" />刷新</Button>
      <Button size="sm" @click="openCreate"><Plus />添加账号</Button>
    </PageHeader>

    <div v-if="!loaded" class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <Skeleton v-for="i in 3" :key="i" class="h-44 rounded-xl" />
    </div>
    <Card v-else-if="!list.length" class="py-0">
      <EmptyState :icon="UserRoundKey" title="还没有 DNS 账号" description="添加 Cloudflare、DNSPod、阿里云等服务商的 API 凭据">
        <Button size="sm" @click="openCreate"><Plus />添加账号</Button>
      </EmptyState>
    </Card>
    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <Card v-for="a in list" :key="a.id">
        <CardHeader>
          <CardTitle class="flex min-w-0 items-center gap-2.5">
            <BrandIcon kind="provider" :type="a.provider" :name="meta.providerName(a.provider)" class="size-7" />
            <span class="truncate" :title="a.name">{{ a.name }}</span>
          </CardTitle>
          <CardDescription class="flex items-center gap-2">
            <ToneBadge tone="primary">{{ meta.providerName(a.provider) }}</ToneBadge>
            <span class="text-xs">{{ a.taskCount ? `${a.taskCount} 个任务在用` : '未被引用' }}</span>
          </CardDescription>
          <CardAction>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="icon" class="-mr-2 size-8"><EllipsisVertical /></Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="w-36">
                <DropdownMenuItem @select="openEdit(a)"><Pencil />编辑</DropdownMenuItem>
                <DropdownMenuItem @select="testSaved(a)"><PlugZap />测试连接</DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem variant="destructive" @select="remove(a)"><Trash2 />删除</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </CardAction>
        </CardHeader>
        <CardContent class="text-muted-foreground line-clamp-2 text-sm break-words">{{ a.remark || '无备注' }}</CardContent>
        <CardFooter class="flex items-center justify-between border-t [.border-t]:pt-3">
          <span class="text-muted-foreground text-xs">更新于 {{ fromNow(a.updatedAt) }}</span>
          <div class="flex gap-1">
            <Button variant="outline" size="sm" :disabled="testing[a.id]" @click="testSaved(a)">
              <Loader2 v-if="testing[a.id]" class="animate-spin" /><PlugZap v-else />测试
            </Button>
            <Button variant="outline" size="sm" @click="openEdit(a)"><Pencil />编辑</Button>
          </div>
        </CardFooter>
      </Card>
    </div>

    <Dialog v-model:open="dlg.open">
      <DialogContent class="flex max-h-[90vh] flex-col gap-0 p-0 sm:max-w-xl">
        <DialogHeader class="border-b px-5 py-3.5">
          <DialogTitle>{{ editing ? '编辑 DNS 账号' : dlg.step === 1 ? '选择 DNS 服务商' : '添加 DNS 账号' }}</DialogTitle>
          <DialogDescription>{{ dlg.step === 1 ? '选择你的域名托管在哪个服务商' : '凭据仅保存在本机，并加密存储' }}</DialogDescription>
        </DialogHeader>

        <div class="flex-1 overflow-y-auto px-5 py-4">
          <TypePicker v-if="dlg.step === 1" kind="provider" :types="meta.providers" @pick="pickProvider" />
          <div v-else class="grid grid-cols-1 gap-3">
            <div v-if="providerMeta" class="bg-muted/40 flex items-start gap-3 rounded-lg border p-3">
              <BrandIcon kind="provider" :type="providerMeta.type" :name="providerMeta.name" class="size-9" />
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2 text-sm font-medium">
                  {{ providerMeta.name }}
                  <button v-if="!editing" type="button" class="text-primary flex items-center gap-0.5 text-xs font-normal hover:underline" @click="dlg.step = 1">
                    <ArrowLeft class="size-3" />更换
                  </button>
                </div>
                <p v-if="providerMeta.description" class="text-muted-foreground mt-1 text-xs leading-relaxed">{{ providerMeta.description }}</p>
              </div>
              <Button v-if="providerMeta.docsUrl" variant="outline" size="sm" as-child>
                <a :href="providerMeta.docsUrl" target="_blank" rel="noopener"><ExternalLink />获取凭据</a>
              </Button>
            </div>
            <FormItem label="名称" required for="acc-name" :error="dlg.nameError">
              <Input id="acc-name" v-model="dlg.name" maxlength="64" placeholder="便于识别的名称" />
            </FormItem>
            <SchemaForm v-if="providerMeta" :key="dlg.key" ref="schemaRef" v-model="dlg.config" :fields="providerMeta.fields" :editing="editing" />
            <Alert v-else variant="destructive"><AlertTitle>未知的服务商类型：{{ dlg.provider }}</AlertTitle></Alert>
            <FormItem label="备注" for="acc-remark">
              <Textarea id="acc-remark" v-model="dlg.remark" maxlength="200" class="min-h-16" />
            </FormItem>
            <Alert v-if="dlg.result" :class="dlg.result.ok ? 'border-success/40 bg-success/5' : 'border-destructive/40 bg-destructive/5'">
              <CircleCheck v-if="dlg.result.ok" class="text-success!" />
              <CircleX v-else class="text-destructive!" />
              <AlertTitle :class="dlg.result.ok ? 'text-success' : 'text-destructive'">{{ dlg.result.ok ? '连接成功' : '连接失败' }}</AlertTitle>
              <AlertDescription v-if="dlg.result.message">{{ dlg.result.message }}</AlertDescription>
            </Alert>
          </div>
        </div>

        <DialogFooter v-if="dlg.step === 2" class="border-t px-5 py-3 sm:justify-between">
          <Button variant="outline" :disabled="dlg.testing || !providerMeta" @click="test">
            <Loader2 v-if="dlg.testing" class="animate-spin" /><PlugZap v-else />测试连接
          </Button>
          <div class="flex gap-2">
            <Button variant="ghost" @click="dlg.open = false">取消</Button>
            <Button :disabled="dlg.saving || !providerMeta" @click="save"><Loader2 v-if="dlg.saving" class="animate-spin" />保存</Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
