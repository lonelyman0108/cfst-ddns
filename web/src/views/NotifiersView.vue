<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { ArrowLeft, Bell, CircleCheck, CircleX, EllipsisVertical, ExternalLink, Loader2, Pencil, Plus, RefreshCw, Send, Trash2 } from '@lucide/vue'
import { notifiersApi } from '@/api'
import type { Notifier, NotifierInput, TestResult, TypeMeta } from '@/api/types'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
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

const { t } = useI18n()
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
  dlg.nameError = dlg.form.name.trim() ? '' : t('notifiers.nameRequired')
  const ok = schemaRef.value?.validate() ?? false
  if (dlg.nameError || !ok) return
  dlg.saving = true
  const body: NotifierInput = { ...dlg.form, name: dlg.form.name.trim(), config: { ...dlg.form.config } }
  try {
    if (editing.value) await notifiersApi.update(dlg.id, body)
    else await notifiersApi.create(body)
    toast.success(t('notifiers.saved'))
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
    if (r.ok) toast.success(t('notifiers.testSent'), { description: r.message || undefined })
    else toast.error(t('notifiers.sendFailed'), { description: r.message || undefined })
  } catch {
    /* 已提示 */
  } finally {
    testing.value[n.id] = false
  }
}

async function remove(n: Notifier) {
  const ok = await confirm({
    title: t('notifiers.deleteTitle', { name: n.name }),
    description: t('notifiers.deleteDescription'),
    confirmText: t('common.delete'),
    destructive: true,
  })
  if (!ok) return
  try {
    await notifiersApi.remove(n.id)
    toast.success(t('notifiers.deleted'))
    load()
  } catch {
    /* 已提示 */
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <PageHeader :title="t('notifiers.title')" :description="t('notifiers.description')">
      <Button variant="outline" size="sm" :disabled="loading" @click="load"><RefreshCw :class="loading ? 'animate-spin' : ''" />{{ t('common.refresh') }}</Button>
      <Button size="sm" @click="openCreate"><Plus />{{ t('notifiers.add') }}</Button>
    </PageHeader>

    <div v-if="!loaded" class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <Skeleton v-for="i in 3" :key="i" class="h-44 rounded-xl" />
    </div>
    <Card v-else-if="!list.length" class="py-0">
      <EmptyState :icon="Bell" :title="t('notifiers.emptyTitle')" :description="t('notifiers.emptyDescription')">
        <Button size="sm" @click="openCreate"><Plus />{{ t('notifiers.add') }}</Button>
      </EmptyState>
    </Card>
    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <Card v-for="n in list" :key="n.id">
        <CardHeader>
          <div class="flex min-w-0 items-center gap-3">
            <BrandIcon kind="notifier" :type="n.type" :name="meta.notifierName(n.type)" :class="n.enabled ? 'size-9' : 'size-9 opacity-50'" />
            <div class="grid min-w-0 gap-0.5">
              <CardTitle class="truncate" :title="n.name">{{ n.name }}</CardTitle>
              <CardDescription class="truncate text-xs">
                {{ [n.name === meta.notifierName(n.type) ? '' : meta.notifierName(n.type), n.enabled ? '' : t('common.disabled')].filter(Boolean).join(' · ') || t('common.enabled') }}
              </CardDescription>
            </div>
          </div>
          <CardAction class="flex items-center gap-1">
            <Switch
              :model-value="n.enabled"
              :disabled="toggling[n.id]"
              :aria-label="n.enabled ? t('common.disable') : t('common.enable')"
              @update:model-value="toggleEnabled(n, $event)"
            />
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="icon" class="-mr-2 size-8"><EllipsisVertical /></Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="w-36">
                <DropdownMenuItem @select="openEdit(n)"><Pencil />{{ t('common.edit') }}</DropdownMenuItem>
                <DropdownMenuItem @select="testSaved(n)"><Send />{{ t('notifiers.sendTest') }}</DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem variant="destructive" @select="remove(n)"><Trash2 />{{ t('common.delete') }}</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </CardAction>
        </CardHeader>
        <CardContent class="flex flex-1 flex-wrap content-start items-center gap-1.5">
          <span class="text-muted-foreground text-xs">{{ t('notifiers.triggerPrefix') }}</span>
          <ToneBadge v-if="n.onSuccess" tone="success">{{ t('notifiers.onSuccessShort') }}</ToneBadge>
          <ToneBadge v-if="n.onFailure" tone="danger">{{ t('notifiers.onFailureShort') }}</ToneBadge>
          <ToneBadge v-if="n.onlyOnChange" tone="warning">{{ t('notifiers.onlyOnChange') }}</ToneBadge>
          <span v-if="!n.onSuccess && !n.onFailure" class="text-muted-foreground text-xs">{{ t('notifiers.never') }}</span>
        </CardContent>
        <CardFooter class="flex items-center justify-between border-t [.border-t]:pt-3">
          <span class="text-muted-foreground text-xs">{{ t('common.updatedAt', { time: fromNow(n.updatedAt) }) }}</span>
          <div class="flex gap-1">
            <Button variant="outline" size="sm" :disabled="testing[n.id]" @click="testSaved(n)">
              <Loader2 v-if="testing[n.id]" class="animate-spin" /><Send v-else />{{ t('common.test') }}
            </Button>
            <Button variant="outline" size="sm" @click="openEdit(n)"><Pencil />{{ t('common.edit') }}</Button>
          </div>
        </CardFooter>
      </Card>
    </div>

    <Dialog v-model:open="dlg.open">
      <DialogContent class="flex max-h-[90vh] flex-col gap-0 p-0 sm:max-w-xl">
        <DialogHeader class="border-b px-5 py-3.5">
          <DialogTitle>{{ editing ? t('notifiers.editTitle') : dlg.step === 1 ? t('notifiers.pickTitle') : t('notifiers.createTitle') }}</DialogTitle>
          <DialogDescription>{{ dlg.step === 1 ? t('notifiers.pickDescription') : t('notifiers.formDescription') }}</DialogDescription>
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
                    <ArrowLeft class="size-3" />{{ t('notifiers.change') }}
                  </button>
                </div>
                <p v-if="typeMeta.description" class="text-muted-foreground mt-1 text-xs leading-relaxed">{{ typeMeta.description }}</p>
              </div>
              <Button v-if="typeMeta.docsUrl" variant="outline" size="sm" as-child>
                <a :href="typeMeta.docsUrl" target="_blank" rel="noopener"><ExternalLink />{{ t('notifiers.docs') }}</a>
              </Button>
            </div>
            <div class="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-[1fr_auto]">
              <FormItem :label="t('common.name')" required for="nt-name" :error="dlg.nameError">
                <Input id="nt-name" v-model="dlg.form.name" maxlength="64" />
              </FormItem>
              <FormItem :label="t('common.enable')">
                <div class="flex h-9 items-center"><Switch v-model="dlg.form.enabled" /></div>
              </FormItem>
            </div>
            <SchemaForm v-if="typeMeta" :key="dlg.key" ref="schemaRef" v-model="dlg.form.config" :fields="typeMeta.fields" :editing="editing" />
            <Alert v-else variant="destructive"><AlertTitle>{{ t('notifiers.unknownType', { type: dlg.form.type }) }}</AlertTitle></Alert>

            <Separator />
            <FormItem :label="t('notifiers.trigger')">
              <div class="flex flex-wrap gap-4">
                <label class="flex cursor-pointer items-center gap-2 text-sm">
                  <Checkbox :model-value="dlg.form.onSuccess" @update:model-value="dlg.form.onSuccess = !!$event" />{{ t('notifiers.onSuccess') }}
                </label>
                <label class="flex cursor-pointer items-center gap-2 text-sm">
                  <Checkbox :model-value="dlg.form.onFailure" @update:model-value="dlg.form.onFailure = !!$event" />{{ t('notifiers.onFailure') }}
                </label>
              </div>
            </FormItem>
            <FormItem :label="t('notifiers.onlyOnChange')" :help="t('notifiers.onlyOnChangeHelp')">
              <div class="flex h-9 items-center"><Switch v-model="dlg.form.onlyOnChange" /></div>
            </FormItem>

            <Alert v-if="dlg.result" :class="dlg.result.ok ? 'border-success/40 bg-success/5' : 'border-destructive/40 bg-destructive/5'">
              <CircleCheck v-if="dlg.result.ok" class="text-success!" />
              <CircleX v-else class="text-destructive!" />
              <AlertTitle :class="dlg.result.ok ? 'text-success' : 'text-destructive'">{{ dlg.result.ok ? t('notifiers.testSent') : t('notifiers.sendFailed') }}</AlertTitle>
              <AlertDescription v-if="dlg.result.message">{{ dlg.result.message }}</AlertDescription>
            </Alert>
          </div>
        </div>

        <DialogFooter v-if="dlg.step === 2" class="border-t px-5 py-3 sm:justify-between">
          <Button variant="outline" :disabled="dlg.testing || !typeMeta" @click="test">
            <Loader2 v-if="dlg.testing" class="animate-spin" /><Send v-else />{{ t('notifiers.sendTest') }}
          </Button>
          <div class="flex gap-2">
            <Button variant="ghost" @click="dlg.open = false">{{ t('common.cancel') }}</Button>
            <Button :disabled="dlg.saving || !typeMeta" @click="save"><Loader2 v-if="dlg.saving" class="animate-spin" />{{ t('common.save') }}</Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
