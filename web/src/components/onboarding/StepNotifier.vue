<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { toast } from 'vue-sonner'
import { ArrowLeft, CircleCheck, CircleX, ExternalLink, Loader2, Plus, Save, Send } from '@lucide/vue'
import { notifiersApi } from '@/api'
import type { Config, Notifier, TestResult, TypeMeta } from '@/api/types'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import BrandIcon from '@/components/BrandIcon.vue'
import FormItem from '@/components/FormItem.vue'
import InlineLink from '@/components/InlineLink.vue'
import SchemaForm, { schemaDefaults } from '@/components/SchemaForm.vue'
import SettingRow from '@/components/SettingRow.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import TypePicker from '@/components/TypePicker.vue'
import { useMetaStore } from '@/stores/meta'

const emit = defineEmits<{ (e: 'changed'): void }>()

const meta = useMetaStore()
const list = ref<Notifier[] | null>(null)
const adding = ref(false)

const form = reactive({
  type: '',
  name: '',
  nameError: '',
  config: {} as Config,
  onSuccess: true,
  onFailure: true,
  onlyOnChange: false,
  testing: false,
  saving: false,
  result: null as TestResult | null,
  key: 0,
})
const schemaRef = ref<InstanceType<typeof SchemaForm>>()
const typeMeta = computed<TypeMeta | undefined>(() => meta.notifiers.find((p) => p.type === form.type))

async function load() {
  const [ns] = await Promise.all([notifiersApi.list().catch(() => []), meta.loadNotifiers().catch(() => {})])
  list.value = ns ?? []
  adding.value = !list.value.length
}

onMounted(load)

function pick(t: TypeMeta) {
  form.type = t.type
  form.config = schemaDefaults(t.fields)
  form.name = t.name
  form.result = null
  form.key++
}

async function test() {
  if (!schemaRef.value?.validate()) return
  form.testing = true
  form.result = null
  try {
    form.result = await notifiersApi.test({ type: form.type, config: { ...form.config } })
  } catch {
    /* 已提示 */
  } finally {
    form.testing = false
  }
}

async function save() {
  form.nameError = form.name.trim() ? '' : '请填写名称'
  if (form.nameError || !schemaRef.value?.validate()) return
  form.saving = true
  try {
    await notifiersApi.create({
      name: form.name.trim(),
      type: form.type,
      enabled: true,
      config: { ...form.config },
      onSuccess: form.onSuccess,
      onFailure: form.onFailure,
      onlyOnChange: form.onlyOnChange,
    })
    toast.success('通知渠道已添加')
    form.type = ''
    await load()
    emit('changed')
  } catch {
    /* 已提示 */
  } finally {
    form.saving = false
  }
}
</script>

<template>
  <div class="grid gap-4">
    <Skeleton v-if="!list" class="h-40" />
    <template v-else>
      <div v-if="list.length" class="grid gap-2">
        <div v-for="n in list" :key="n.id" class="flex items-center gap-3 rounded-lg border px-3 py-2.5">
          <BrandIcon kind="notifier" :type="n.type" class="size-7" />
          <span class="min-w-0 flex-1 truncate text-sm font-medium">{{ n.name }}</span>
          <CircleCheck class="text-success size-4 shrink-0" />
          <ToneBadge tone="primary">{{ meta.notifierName(n.type) }}</ToneBadge>
        </div>
        <div v-if="!adding">
          <Button variant="outline" size="sm" @click="adding = true"><Plus />再添加一个</Button>
        </div>
      </div>

      <template v-if="adding">
        <TypePicker v-if="!form.type" kind="notifier" :types="meta.notifiers" @pick="pick" />
        <div v-else class="grid gap-3">
          <div v-if="typeMeta" class="bg-muted/40 flex flex-wrap items-start gap-3 rounded-lg border p-3">
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2 text-sm font-medium">
                {{ typeMeta.name }}
                <InlineLink :icon="ArrowLeft" class="text-xs font-normal" @click="form.type = ''">更换</InlineLink>
              </div>
              <p v-if="typeMeta.description" class="text-muted-foreground mt-1 text-xs leading-relaxed">{{ typeMeta.description }}</p>
            </div>
            <Button v-if="typeMeta.docsUrl" variant="outline" size="sm" as-child>
              <a :href="typeMeta.docsUrl" target="_blank" rel="noopener"><ExternalLink />配置说明</a>
            </Button>
          </div>
          <FormItem label="名称" required for="ob-ntf-name" :error="form.nameError">
            <Input id="ob-ntf-name" v-model="form.name" maxlength="64" />
          </FormItem>
          <SchemaForm v-if="typeMeta" :key="form.key" ref="schemaRef" v-model="form.config" :fields="typeMeta.fields" />
          <div class="divide-y rounded-lg border">
            <SettingRow label="成功时通知" for="ob-on-success"><Switch id="ob-on-success" v-model="form.onSuccess" /></SettingRow>
            <SettingRow label="失败时通知" for="ob-on-failure"><Switch id="ob-on-failure" v-model="form.onFailure" /></SettingRow>
            <SettingRow label="仅 IP 变化时通知" description="IP 没变时不打扰" for="ob-on-change">
              <Switch id="ob-on-change" v-model="form.onlyOnChange" />
            </SettingRow>
          </div>
          <Alert v-if="form.result" :class="form.result.ok ? 'border-success/40 bg-success/5' : 'border-destructive/40 bg-destructive/5'">
            <CircleCheck v-if="form.result.ok" class="text-success!" />
            <CircleX v-else class="text-destructive!" />
            <AlertTitle :class="form.result.ok ? 'text-success' : 'text-destructive'">{{ form.result.ok ? '已发送测试消息' : '发送失败' }}</AlertTitle>
            <AlertDescription v-if="form.result.message">{{ form.result.message }}</AlertDescription>
          </Alert>
          <div class="flex flex-wrap gap-2">
            <Button variant="outline" :disabled="form.testing" @click="test">
              <Loader2 v-if="form.testing" class="animate-spin" /><Send v-else />发送测试
            </Button>
            <Button :disabled="form.saving" @click="save"><Loader2 v-if="form.saving" class="animate-spin" /><Save v-else />保存渠道</Button>
          </div>
        </div>
      </template>
    </template>
  </div>
</template>
