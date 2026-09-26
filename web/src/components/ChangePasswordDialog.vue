<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Loader2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { useAuthStore } from '@/stores/auth'
import FormItem from './FormItem.vue'

const { t } = useI18n()
const open = defineModel<boolean>({ default: false })
const auth = useAuthStore()

const saving = ref(false)
const form = reactive({ oldPassword: '', newPassword: '', confirm: '' })
const errors = reactive<Record<string, string>>({})

watch(open, (v) => {
  if (v) {
    form.oldPassword = form.newPassword = form.confirm = ''
    for (const k of Object.keys(errors)) delete errors[k]
  }
})

function validate() {
  for (const k of Object.keys(errors)) delete errors[k]
  if (!form.oldPassword) errors.oldPassword = t('components.changePassword.oldRequired')
  if (form.newPassword.length < 6) errors.newPassword = t('components.changePassword.tooShort')
  if (form.confirm !== form.newPassword) errors.confirm = t('components.changePassword.mismatch')
  return !Object.keys(errors).length
}

async function submit() {
  if (!validate()) return
  saving.value = true
  try {
    // 后端会使旧令牌失效并返回新令牌，store 内完成替换
    await auth.changePassword({ oldPassword: form.oldPassword, newPassword: form.newPassword })
    toast.success(t('components.changePassword.done'))
    open.value = false
  } catch {
    /* 已提示 */
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t('components.changePassword.title') }}</DialogTitle>
        <DialogDescription>{{ t('components.changePassword.description') }}</DialogDescription>
      </DialogHeader>
      <form class="grid grid-cols-1 gap-4" @submit.prevent="submit">
        <FormItem :label="t('components.changePassword.old')" for="pw-old" :error="errors.oldPassword">
          <Input id="pw-old" v-model="form.oldPassword" type="password" autocomplete="current-password" />
        </FormItem>
        <FormItem :label="t('components.changePassword.new')" for="pw-new" :error="errors.newPassword">
          <Input id="pw-new" v-model="form.newPassword" type="password" autocomplete="new-password" />
        </FormItem>
        <FormItem :label="t('components.changePassword.confirm')" for="pw-confirm" :error="errors.confirm">
          <Input id="pw-confirm" v-model="form.confirm" type="password" autocomplete="new-password" />
        </FormItem>
        <DialogFooter>
          <Button type="button" variant="outline" @click="open = false">{{ t('common.cancel') }}</Button>
          <Button type="submit" :disabled="saving"><Loader2 v-if="saving" class="animate-spin" />{{ t('common.save') }}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
