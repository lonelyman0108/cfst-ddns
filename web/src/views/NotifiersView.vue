<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { toast } from 'vue-sonner'
import { ArrowLeft, Bell, CircleCheck, CircleX, EllipsisVertical, ExternalLink, Loader2, Pencil, Plus, RefreshCw, Send, Trash2 } from '@lucide/vue'
import { notifiersApi } from '@/api'
import type { Notifier, NotifierInput, TestResult, TypeMeta } from '@/api/types'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import EmptyState from '@/components/EmptyState.vue'
import FormItem from '@/components/FormItem.vue'
import PageHeader from '@/components/PageHeader.vue'
import SchemaForm, { mergeSchemaDefaults, schemaDefaults } from '@/components/SchemaForm.vue'
import BrandIcon from '@/components/BrandIcon.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import TypePicker from '@/components/TypePicker.vue'
import { confirm } from '@/composables/useConfirm'
import { useMetaStore } from '@/stores/meta'

const meta = useMetaStore()
const list = ref<Notifier[]>([])
const loading = ref(false)
const loaded = ref(false)
const testing = ref<Record<number, boolean>>({})
const toggling = ref<Record<number, boolean>>({})

async function load() {
  loading.value = true
  try {
    const [ns] = await Promise.all([notifiersApi.list(), meta.loadNotifiers().catch(() => {})])
    list.value = ns ?? []
    loaded.value = true
  } catch {
    /* 已提示 */
  } finally {
    loading.value = false
  }
}

onMounted(load)

function toInput(n: Notifier): NotifierInput {
  return {
    name: n.name,
    type: n.type,
    enabled: n.enabled,
    config: { ...(n.config ?? {}) },
    onSuccess: n.onSuccess,
    onFailure: n.onFailure,
    onlyOnChange: n.onlyOnChange,
  }
}

async function toggleEnabled(n: Notifier, v: boolean) {
  toggling.value[n.id] = true
  try {
    // 密钥字段为 ******，后端按契约保持原值
    Object.assign(n, await notifiersApi.update(n.id, { ...toInput(n), enabled: v }))
  } catch {
    /* 已提示 */
  } finally {
    toggling.value[n.id] = false
  }
}

// ---------- 对话框 ----------
const emptyForm = (): NotifierInput => ({
  name: '',
  type: '',
  enabled: true,
  config: {},
  onSuccess: true,
  onFailure: true,
  onlyOnChange: false,
})

const dlg = reactive({
  open: false,
  step: 1 as 1 | 2,
  id: 0,
  form: emptyForm(),
  nameError: '',
  saving: false,
  testing: false,
  result: null as TestResult | null,
  key: 0,
})
const schemaRef = ref<InstanceType<typeof SchemaForm>>()

const typeMeta = computed<TypeMeta | undefined>(() => meta.notifiers.find((p) => p.type === dlg.form.type))
const editing = computed(() => dlg.id > 0)

function openCreate() {
  Object.assign(dlg, { open: true, step: 1, id: 0, form: emptyForm(), nameError: '', result: null })
  dlg.key++
}

function pickType(t: TypeMeta) {
  dlg.form.type = t.type
  dlg.form.config = schemaDefaults(t.fields)
  if (!dlg.form.name) dlg.form.name = t.name
  dlg.step = 2
  dlg.result = null
  dlg.key++
}

function openEdit(n: Notifier) {
  const tm = meta.notifiers.find((p) => p.type === n.type)
  const form = toInput(n)
  if (tm) form.config = mergeSchemaDefaults(tm.fields, n.config)
  Object.assign(dlg, { open: true, step: 2, id: n.id, form, nameError: '', result: null })
  dlg.key++
}

async function test() {
  if (!schemaRef.value?.validate()) return
  dlg.testing = true
  dlg.result = null
  try {
    // 编辑时带上 id：未修改的密钥（******）由后端用已保存值补齐，其余字段按表单测试
    dlg.result = await notifiersApi.test({
      type: dlg.form.type,
      config: { ...dlg.form.config },
      ...(editing.value ? { id: dlg.id } : {}),
    })
  } catch {
    /* 已提示 */
  } finally {
    dlg.testing = false
  }
}

async function save() {
  dlg.nameError = dlg.form.name.trim() ? '' : '请填写名称'
  const ok = schemaRef.value?.validate() ?? false
  if (dlg.nameError || !ok) return
  dlg.saving = true
  const body: NotifierInput = { ...dlg.form, name: dlg.form.name.trim(), config: { ...dlg.form.config } }
  try {
    if (editing.value) await notifiersApi.update(dlg.id, body)
    else await notifiersApi.create(body)
    toast.success('通知渠道已保存')
    dlg.open = false
    load()
  } catch {
    /* 已提示 */
  } finally {
    dlg.saving = false
  }
}

async function testSaved(n: Notifier) {
  testing.value[n.id] = true
  try {
    const r = await notifiersApi.testSaved(n.id)
    if (r.ok) toast.success('测试消息已发送', { description: r.message || undefined })
    else toast.error('发送失败', { description: r.message || undefined })
  } catch {
    /* 已提示 */
  } finally {
    testing.value[n.id] = false
  }
}

async function remove(n: Notifier) {
  const ok = await confirm({
    title: `删除通知渠道「${n.name}」？`,
    description: '引用它的任务将不再发送该渠道的通知。',
    confirmText: '删除',
    destructive: true,
  })
  if (!ok) return
  try {
    await notifiersApi.remove(n.id)
    toast.success('通知渠道已删除')
    load()
  } catch {
    /* 已提示 */
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader title="通知渠道" description="任务执行完成后推送结果">
      <Button variant="outline" size="sm" :disabled="loading" @click="load"><RefreshCw :class="loading ? 'animate-spin' : ''" />刷新</Button>
      <Button size="sm" @click="openCreate"><Plus />添加通知渠道</Button>
    </PageHeader>

    <Card class="gap-0 overflow-hidden py-0">
      <div v-if="!loaded" class="grid grid-cols-1 gap-3 p-5"><Skeleton v-for="i in 3" :key="i" class="h-10" /></div>
      <EmptyState v-else-if="!list.length" :icon="Bell" title="还没有通知渠道" description="支持 Bark、Telegram、企业微信、钉钉、飞书、邮件等">
        <Button size="sm" @click="openCreate"><Plus />添加通知渠道</Button>
      </EmptyState>
      <Table v-else>
        <TableHeader>
          <TableRow class="bg-muted/30 hover:bg-muted/30">
            <TableHead class="pl-5">名称</TableHead>
            <TableHead>类型</TableHead>
            <TableHead class="w-16">启用</TableHead>
            <TableHead>触发条件</TableHead>
            <TableHead class="w-32 pr-5 text-right">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="n in list" :key="n.id">
            <TableCell class="min-w-48 pl-5 font-medium whitespace-normal">
              <div class="flex items-center gap-2.5">
                <BrandIcon kind="notifier" :type="n.type" :name="meta.notifierName(n.type)" class="size-7 shrink-0" />
                <span class="line-clamp-2 min-w-0 wrap-anywhere" :title="n.name">{{ n.name }}</span>
              </div>
            </TableCell>
            <TableCell><ToneBadge tone="primary">{{ meta.notifierName(n.type) }}</ToneBadge></TableCell>
            <TableCell><Switch :model-value="n.enabled" :disabled="toggling[n.id]" @update:model-value="toggleEnabled(n, $event)" /></TableCell>
            <TableCell>
              <div class="flex flex-wrap gap-1">
                <ToneBadge v-if="n.onSuccess" tone="success">成功</ToneBadge>
                <ToneBadge v-if="n.onFailure" tone="danger">失败</ToneBadge>
                <ToneBadge v-if="n.onlyOnChange" tone="warning">仅 IP 变化时</ToneBadge>
                <span v-if="!n.onSuccess && !n.onFailure" class="text-muted-foreground text-xs">不触发</span>
              </div>
            </TableCell>
            <TableCell class="pr-5">
              <div class="flex items-center justify-end gap-1">
                <Button variant="ghost" size="sm" :disabled="testing[n.id]" @click="testSaved(n)">
                  <Loader2 v-if="testing[n.id]" class="animate-spin" /><Send v-else />测试
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <Button variant="ghost" size="icon" class="size-8"><EllipsisVertical /></Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" class="w-32">
                    <DropdownMenuItem @select="openEdit(n)"><Pencil />编辑</DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem variant="destructive" @select="remove(n)"><Trash2 />删除</DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </Card>

    <Dialog v-model:open="dlg.open">
      <DialogContent class="flex max-h-[90vh] flex-col gap-0 p-0 sm:max-w-xl">
        <DialogHeader class="border-b px-5 py-3.5">
          <DialogTitle>{{ editing ? '编辑通知渠道' : dlg.step === 1 ? '选择通知类型' : '添加通知渠道' }}</DialogTitle>
          <DialogDescription>{{ dlg.step === 1 ? '选择接收通知的平台' : '配置推送参数与触发条件' }}</DialogDescription>
        </DialogHeader>

        <div class="flex-1 overflow-y-auto px-5 py-4">
          <TypePicker v-if="dlg.step === 1" kind="notifier" :types="meta.notifiers" @pick="pickType" />
          <div v-else class="grid grid-cols-1 gap-3">
            <div v-if="typeMeta" class="bg-muted/40 flex items-start gap-3 rounded-lg border p-3">
              <BrandIcon kind="notifier" :type="typeMeta.type" :name="typeMeta.name" class="size-9" />
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2 text-sm font-medium">
                  {{ typeMeta.name }}
                  <button v-if="!editing" type="button" class="text-primary flex items-center gap-0.5 text-xs font-normal hover:underline" @click="dlg.step = 1">
                    <ArrowLeft class="size-3" />更换
                  </button>
                </div>
                <p v-if="typeMeta.description" class="text-muted-foreground mt-1 text-xs leading-relaxed">{{ typeMeta.description }}</p>
              </div>
              <Button v-if="typeMeta.docsUrl" variant="outline" size="sm" as-child>
                <a :href="typeMeta.docsUrl" target="_blank" rel="noopener"><ExternalLink />文档</a>
              </Button>
            </div>
            <div class="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-[1fr_auto]">
              <FormItem label="名称" required for="nt-name" :error="dlg.nameError">
                <Input id="nt-name" v-model="dlg.form.name" maxlength="64" />
              </FormItem>
              <FormItem label="启用">
                <div class="flex h-9 items-center"><Switch v-model="dlg.form.enabled" /></div>
              </FormItem>
            </div>
            <SchemaForm v-if="typeMeta" :key="dlg.key" ref="schemaRef" v-model="dlg.form.config" :fields="typeMeta.fields" :editing="editing" />
            <Alert v-else variant="destructive"><AlertTitle>未知的通知类型：{{ dlg.form.type }}</AlertTitle></Alert>

            <Separator />
            <FormItem label="触发条件">
              <div class="flex flex-wrap gap-4">
                <label class="flex cursor-pointer items-center gap-2 text-sm">
                  <Checkbox :model-value="dlg.form.onSuccess" @update:model-value="dlg.form.onSuccess = !!$event" />执行成功时
                </label>
                <label class="flex cursor-pointer items-center gap-2 text-sm">
                  <Checkbox :model-value="dlg.form.onFailure" @update:model-value="dlg.form.onFailure = !!$event" />执行失败时
                </label>
              </div>
            </FormItem>
            <FormItem label="仅 IP 变化时" help="只在本次执行实际修改了 DNS 记录时通知，避免重复提醒">
              <div class="flex h-9 items-center"><Switch v-model="dlg.form.onlyOnChange" /></div>
            </FormItem>

            <Alert v-if="dlg.result" :class="dlg.result.ok ? 'border-success/40 bg-success/5' : 'border-destructive/40 bg-destructive/5'">
              <CircleCheck v-if="dlg.result.ok" class="text-success!" />
              <CircleX v-else class="text-destructive!" />
              <AlertTitle :class="dlg.result.ok ? 'text-success' : 'text-destructive'">{{ dlg.result.ok ? '测试消息已发送' : '发送失败' }}</AlertTitle>
              <AlertDescription v-if="dlg.result.message">{{ dlg.result.message }}</AlertDescription>
            </Alert>
          </div>
        </div>

        <DialogFooter v-if="dlg.step === 2" class="border-t px-5 py-3 sm:justify-between">
          <Button variant="outline" :disabled="dlg.testing || !typeMeta" @click="test">
            <Loader2 v-if="dlg.testing" class="animate-spin" /><Send v-else />发送测试
          </Button>
          <div class="flex gap-2">
            <Button variant="ghost" @click="dlg.open = false">取消</Button>
            <Button :disabled="dlg.saving || !typeMeta" @click="save"><Loader2 v-if="dlg.saving" class="animate-spin" />保存</Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
