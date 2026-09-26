<script lang="ts">
import type { Config, Field } from '@/api/types'

export const SECRET_MASK = '******'

/** 根据字段 Schema 生成初始配置（default 优先，switch 默认 "false"） */
export function schemaDefaults(fields: Field[]): Config {
  const c: Config = {}
  for (const f of fields) {
    if (f.default !== undefined && f.default !== null) c[f.key] = String(f.default)
    else if (f.type === 'switch') c[f.key] = 'false'
    else c[f.key] = ''
  }
  return c
}

/** 用 Schema 默认值补齐已有配置中缺失的键 */
export function mergeSchemaDefaults(fields: Field[], config: Config | null | undefined): Config {
  return { ...schemaDefaults(fields), ...(config ?? {}) }
}
</script>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { Eye, EyeOff } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import FormItem from './FormItem.vue'
import NumInput from './NumInput.vue'

const props = withDefaults(
  defineProps<{
    fields: Field[]
    /** 编辑已保存的对象：值为 ****** 的密钥字段显示为占位，未修改则原样提交 */
    editing?: boolean
  }>(),
  { editing: false },
)

const model = defineModel<Config>({ required: true })
const { t } = useI18n()

const errors = reactive<Record<string, string>>({})
const reveal = reactive<Record<string, boolean>>({})

// 挂载时快照“原本即为掩码”的密钥字段；父组件切换对象时通过 :key 重建本组件
const masked = ref(
  new Set(props.editing ? props.fields.filter((f) => f.secret && model.value[f.key] === SECRET_MASK).map((f) => f.key) : []),
)

function isVisible(f: Field) {
  if (!f.showIf) return true
  return (model.value[f.showIf.key] ?? '') === f.showIf.value
}

const visibleFields = computed(() => props.fields.filter(isVisible))

function textValue(f: Field): string {
  const v = model.value[f.key] ?? ''
  return masked.value.has(f.key) && v === SECRET_MASK ? '' : v
}

function setText(f: Field, v: string | number) {
  const s = String(v ?? '')
  model.value[f.key] = masked.value.has(f.key) && s === '' ? SECRET_MASK : s
  if (errors[f.key]) delete errors[f.key]
}

function numberValue(f: Field): number | undefined {
  const v = model.value[f.key]
  if (v === undefined || v === '') return undefined
  const n = Number(v)
  return Number.isFinite(n) ? n : undefined
}

function setNumber(f: Field, v: number | undefined) {
  model.value[f.key] = v === undefined ? '' : String(v)
  if (errors[f.key]) delete errors[f.key]
}

function placeholderOf(f: Field) {
  if (masked.value.has(f.key)) return t('format.form.secretPlaceholder')
  return f.placeholder || ''
}

function validate(): boolean {
  for (const k of Object.keys(errors)) delete errors[k]
  let ok = true
  for (const f of visibleFields.value) {
    if (!f.required || f.type === 'switch') continue
    if (!(model.value[f.key] ?? '').trim()) {
      errors[f.key] = t('format.form.required', { label: f.label })
      ok = false
    }
  }
  return ok
}

defineExpose({ validate })
</script>

<template>
  <div class="grid grid-cols-1 gap-3">
    <FormItem
      v-for="f in visibleFields"
      :key="f.key"
      :label="f.label"
      :for="`sf-${f.key}`"
      :required="f.required && f.type !== 'switch'"
      :help="f.help"
      :error="errors[f.key]"
    >
      <Input
        v-if="f.type === 'text'"
        :id="`sf-${f.key}`"
        :model-value="textValue(f)"
        :placeholder="placeholderOf(f)"
        :aria-invalid="!!errors[f.key]"
        @update:model-value="setText(f, $event)"
      />
      <div v-else-if="f.type === 'password'" class="relative">
        <Input
          :id="`sf-${f.key}`"
          :type="reveal[f.key] ? 'text' : 'password'"
          :model-value="textValue(f)"
          :placeholder="placeholderOf(f)"
          autocomplete="new-password"
          class="pr-9 font-mono"
          :aria-invalid="!!errors[f.key]"
          @update:model-value="setText(f, $event)"
        />
        <button
          type="button"
          tabindex="-1"
          class="text-muted-foreground hover:text-foreground absolute inset-y-0 right-0 flex w-9 items-center justify-center"
          @click="reveal[f.key] = !reveal[f.key]"
        >
          <EyeOff v-if="reveal[f.key]" class="size-4" />
          <Eye v-else class="size-4" />
        </button>
      </div>
      <Textarea
        v-else-if="f.type === 'textarea'"
        :id="`sf-${f.key}`"
        :model-value="textValue(f)"
        :placeholder="placeholderOf(f)"
        class="min-h-20 font-mono text-code"
        :aria-invalid="!!errors[f.key]"
        @update:model-value="setText(f, $event)"
      />
      <NumInput
        v-else-if="f.type === 'number'"
        :id="`sf-${f.key}`"
        :model-value="numberValue(f)"
        class="max-w-56"
        @update:model-value="setNumber(f, $event)"
      />
      <Switch
        v-else-if="f.type === 'switch'"
        :id="`sf-${f.key}`"
        :model-value="model[f.key] === 'true'"
        @update:model-value="model[f.key] = $event ? 'true' : 'false'"
      />
      <Select
        v-else-if="f.type === 'select'"
        :model-value="model[f.key] || undefined"
        @update:model-value="setText(f, String($event ?? ''))"
      >
        <SelectTrigger :id="`sf-${f.key}`" class="w-full" :aria-invalid="!!errors[f.key]">
          <SelectValue :placeholder="f.placeholder || t('format.form.select')" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="o in f.options || []" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
        </SelectContent>
      </Select>
      <Input v-else :id="`sf-${f.key}`" :model-value="textValue(f)" @update:model-value="setText(f, $event)" />
    </FormItem>
  </div>
</template>
