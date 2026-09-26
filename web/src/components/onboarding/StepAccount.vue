<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { ArrowLeft, CircleCheck, CircleX, ExternalLink, Loader2, PlugZap, Plus, Save } from '@lucide/vue'
import { accountsApi } from '@/api'
import type { Account, Config, TestResult, TypeMeta } from '@/api/types'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import BrandIcon from '@/components/BrandIcon.vue'
import FormItem from '@/components/FormItem.vue'
import InlineLink from '@/components/InlineLink.vue'
import SchemaForm, { schemaDefaults } from '@/components/SchemaForm.vue'
import ToneBadge from '@/components/ToneBadge.vue'
import TypePicker from '@/components/TypePicker.vue'
import { useMetaStore } from '@/stores/meta'

const emit = defineEmits<{ (e: 'changed'): void }>()

const meta = useMetaStore()
const list = ref<Account[] | null>(null)
const adding = ref(false)

const form = reactive({
  provider: '',
  name: '',
  nameError: '',
  config: {} as Config,
  testing: false,
  saving: false,
  result: null as TestResult | null,
  key: 0,
})
const schemaRef = ref<InstanceType<typeof SchemaForm>>()
const providerMeta = computed<TypeMeta | undefined>(() => meta.providers.find((p) => p.type === form.provider))

async function load() {
  const [accs] = await Promise.all([accountsApi.list().catch(() => []), meta.loadProviders().catch(() => {})])
  list.value = accs ?? []
  adding.value = !list.value.length
}

onMounted(load)

function pick(p: TypeMeta) {
  form.provider = p.type
  form.config = schemaDefaults(p.fields)
  form.name = p.name
  form.result = null
  form.key++
}

// 凭据修改后需要重新测试
watch(
  () => JSON.stringify(form.config),
  () => {
    if (form.result?.ok) form.result = null
  },
)

async function test() {
  if (!schemaRef.value?.validate()) return
  form.testing = true
  form.result = null
  try {
    form.result = await accountsApi.test({ provider: form.provider, config: { ...form.config } })
  } catch {
    /* 已提示 */
  } finally {
    form.testing = false
  }
}

async function save() {
  form.nameError = form.name.trim() ? '' : '请填写名称'
  if (form.nameError || !schemaRef.value?.validate() || !form.result?.ok) return
  form.saving = true
  try {
    await accountsApi.create({ name: form.name.trim(), provider: form.provider, config: { ...form.config }, remark: '' })
    toast.success('DNS 账号已添加')
    form.provider = ''
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
  <div class="grid grid-cols-1 gap-4">
    <Skeleton v-if="!list" class="h-40" />
    <template v-else>
      <div v-if="list.length" class="grid grid-cols-1 gap-2">
        <div v-for="a in list" :key="a.id" class="flex items-center gap-3 rounded-lg border px-3 py-2.5">
          <BrandIcon kind="provider" :type="a.provider" class="size-7" />
          <span class="min-w-0 flex-1 truncate text-sm font-medium" :title="a.name">{{ a.name }}</span>
          <CircleCheck class="text-success size-4 shrink-0" />
          <ToneBadge tone="primary">{{ meta.providerName(a.provider) }}</ToneBadge>
        </div>
        <div v-if="!adding">
          <Button variant="outline" size="sm" @click="adding = true"><Plus />再添加一个</Button>
        </div>
      </div>

      <template v-if="adding">
        <TypePicker v-if="!form.provider" kind="provider" :types="meta.providers" @pick="pick" />
        <div v-else class="grid grid-cols-1 gap-3">
          <div v-if="providerMeta" class="bg-muted/40 flex flex-wrap items-start gap-3 rounded-lg border p-3">
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2 text-sm font-medium">
                {{ providerMeta.name }}
                <InlineLink :icon="ArrowLeft" class="text-xs font-normal" @click="form.provider = ''">更换</InlineLink>
              </div>
              <p v-if="providerMeta.description" class="text-muted-foreground mt-1 text-xs leading-relaxed">{{ providerMeta.description }}</p>
            </div>
            <Button v-if="providerMeta.docsUrl" variant="outline" size="sm" as-child>
              <a :href="providerMeta.docsUrl" target="_blank" rel="noopener"><ExternalLink />如何获取凭据</a>
            </Button>
          </div>
          <FormItem label="名称" required for="ob-acc-name" :error="form.nameError">
            <Input id="ob-acc-name" v-model="form.name" maxlength="64" placeholder="便于识别的名称" />
          </FormItem>
          <SchemaForm v-if="providerMeta" :key="form.key" ref="schemaRef" v-model="form.config" :fields="providerMeta.fields" />
          <Alert v-if="form.result" :class="form.result.ok ? 'border-success/40 bg-success/5' : 'border-destructive/40 bg-destructive/5'">
            <CircleCheck v-if="form.result.ok" class="text-success!" />
            <CircleX v-else class="text-destructive!" />
            <AlertTitle :class="form.result.ok ? 'text-success' : 'text-destructive'">{{ form.result.ok ? '连接成功' : '连接失败' }}</AlertTitle>
            <AlertDescription v-if="form.result.message">{{ form.result.message }}</AlertDescription>
          </Alert>
          <div class="flex flex-wrap items-center gap-2">
            <Button variant="outline" :disabled="form.testing" @click="test">
              <Loader2 v-if="form.testing" class="animate-spin" /><PlugZap v-else />测试连接
            </Button>
            <Button :disabled="form.saving || !form.result?.ok" @click="save">
              <Loader2 v-if="form.saving" class="animate-spin" /><Save v-else />保存账号
            </Button>
            <span v-if="!form.result?.ok" class="text-muted-foreground text-xs">测试连接成功后才能保存</span>
          </div>
        </div>
      </template>
    </template>
  </div>
</template>
