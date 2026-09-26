<script setup lang="ts">
import AppLogo from '@/components/AppLogo.vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { FileJson, FileText, Loader2, Moon, RefreshCw, ServerCrash, Sun, X } from '@lucide/vue'
import { backupApi } from '@/api'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import FormItem from '@/components/FormItem.vue'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'

const auth = useAuthStore()
const theme = useThemeStore()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const loading = ref(false)
const statusLoading = ref(true)
const statusError = ref(false)
const form = reactive({ username: '', password: '', confirm: '' })
// 存文案 key，渲染时 t()，切换语言后提示随之更新
const errors = reactive<Record<string, string>>({})

const isSetup = computed(() => auth.initialized === false)

// 初始化后的去向：引导向导 / 从备份恢复 / 从 v1 导入
const after = ref<'welcome' | 'restore' | 'legacy'>('welcome')
const backup = ref<{ name: string; data: unknown } | null>(null)
const backupInput = ref<HTMLInputElement>()

async function onBackupFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  try {
    backup.value = { name: file.name, data: JSON.parse(await file.text()) }
    after.value = 'restore'
  } catch {
    toast.error(t('auth.toast.invalidJson'))
  }
}

function clearAfter() {
  after.value = 'welcome'
  backup.value = null
}

async function loadStatus() {
  statusLoading.value = true
  statusError.value = false
  try {
    await auth.fetchStatus()
  } catch {
    statusError.value = true
  } finally {
    statusLoading.value = false
  }
}

onMounted(loadStatus)

function validate() {
  for (const k of Object.keys(errors)) delete errors[k]
  const u = form.username.trim()
  if (!u) errors.username = 'auth.errors.usernameRequired'
  else if (isSetup.value && (u.length < 3 || u.length > 32)) errors.username = 'auth.errors.usernameLength'
  if (!form.password) errors.password = 'auth.errors.passwordRequired'
  else if (isSetup.value && form.password.length < 6) errors.password = 'auth.errors.passwordMin'
  if (isSetup.value && form.confirm !== form.password) errors.confirm = 'auth.errors.passwordMismatch'
  return !Object.keys(errors).length
}

async function submit() {
  if (!validate()) return
  loading.value = true
  try {
    const cred = { username: form.username.trim(), password: form.password }
    if (isSetup.value) {
      await auth.setup(cred)
      if (after.value === 'restore' && backup.value) {
        try {
          await backupApi.restore(backup.value.data)
          toast.success(t('auth.toast.restored'))
          router.replace('/')
        } catch {
          // 管理员已创建，恢复失败时去设置页的备份区重试
          toast.warning(t('auth.toast.restoreFailed'), { description: t('auth.toast.restoreFailedHint') })
          router.replace('/settings#backup')
        }
        return
      }
      toast.success(t('auth.toast.created'))
      router.replace(after.value === 'legacy' ? '/import/legacy' : '/welcome')
      return
    }
    await auth.login(cred)
    const r = route.query.redirect
    router.replace(typeof r === 'string' && r.startsWith('/') ? r : '/')
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="relative flex min-h-full flex-col items-center justify-center overflow-hidden px-4 py-10">
    <!-- 网格 + 渐变背景 -->
    <div
      class="pointer-events-none absolute inset-0 [background-image:linear-gradient(to_right,var(--border)_1px,transparent_1px),linear-gradient(to_bottom,var(--border)_1px,transparent_1px)] [background-size:44px_44px] [mask-image:radial-gradient(ellipse_at_center,black_20%,transparent_70%)] opacity-60"
    />
    <div class="bg-primary/20 pointer-events-none absolute -top-40 left-1/2 size-[520px] -translate-x-1/2 rounded-full blur-3xl" />

    <div class="absolute top-4 right-4 flex items-center gap-1">
      <LanguageSwitcher />
      <Button variant="ghost" size="icon" class="size-8" @click="theme.toggle()">
        <Sun v-if="theme.dark" />
        <Moon v-else />
      </Button>
    </div>

    <div class="relative z-10 w-full max-w-sm">
      <div class="mb-8 flex flex-col items-center text-center">
        <AppLogo class="shadow-primary/25 mb-4 size-14 rounded-[13px] shadow-lg" />
        <h1 class="text-2xl font-semibold tracking-[-0.01em]">cfst-ddns</h1>
        <p class="text-muted-foreground mt-1.5 text-sm">{{ t('auth.tagline') }}</p>
      </div>

      <Card class="shadow-lg shadow-black/5">
        <template v-if="statusLoading">
          <CardHeader><Skeleton class="h-5 w-24" /><Skeleton class="h-4 w-48" /></CardHeader>
          <CardContent class="grid grid-cols-1 gap-4"><Skeleton class="h-9" /><Skeleton class="h-9" /><Skeleton class="h-9" /></CardContent>
        </template>
        <template v-else-if="statusError">
          <CardContent class="flex flex-col items-center gap-3 py-6 text-center">
            <ServerCrash class="text-muted-foreground size-10" />
            <div class="font-medium">{{ t('auth.serverDown') }}</div>
            <p class="text-muted-foreground text-sm">{{ t('auth.serverDownHint') }}</p>
            <Button variant="outline" size="sm" @click="loadStatus"><RefreshCw />{{ t('common.retry') }}</Button>
          </CardContent>
        </template>
        <template v-else>
          <CardHeader>
            <CardTitle>{{ isSetup ? t('auth.setupTitle') : t('auth.loginTitle') }}</CardTitle>
            <CardDescription>{{ isSetup ? t('auth.setupDesc') : t('auth.loginDesc') }}</CardDescription>
          </CardHeader>
          <CardContent>
            <form class="grid grid-cols-1 gap-4" @submit.prevent="submit">
              <FormItem :label="t('auth.username')" for="username" :error="errors.username && t(errors.username)">
                <Input id="username" v-model="form.username" autocomplete="username" autofocus :aria-invalid="!!errors.username" />
              </FormItem>
              <FormItem :label="t('auth.password')" for="password" :error="errors.password && t(errors.password)">
                <Input
                  id="password"
                  v-model="form.password"
                  type="password"
                  :autocomplete="isSetup ? 'new-password' : 'current-password'"
                  :aria-invalid="!!errors.password"
                />
              </FormItem>
              <FormItem v-if="isSetup" :label="t('auth.confirmPassword')" for="confirm" :error="errors.confirm && t(errors.confirm)">
                <Input id="confirm" v-model="form.confirm" type="password" autocomplete="new-password" :aria-invalid="!!errors.confirm" />
              </FormItem>
              <Button type="submit" class="mt-1 w-full" :disabled="loading">
                <Loader2 v-if="loading" class="animate-spin" />
                {{ isSetup ? (after === 'restore' ? t('auth.createAndRestore') : t('auth.createAndLogin')) : t('auth.login') }}
              </Button>
            </form>
            <template v-if="isSetup">
              <div v-if="after !== 'welcome'" class="bg-muted/40 mt-4 flex items-center gap-2 rounded-md border px-3 py-2 text-sm">
                <FileJson v-if="after === 'restore'" class="text-muted-foreground size-4 shrink-0" />
                <FileText v-else class="text-muted-foreground size-4 shrink-0" />
                <span class="min-w-0 flex-1 truncate">{{ after === 'restore' ? t('auth.afterRestore', { name: backup?.name ?? '' }) : t('auth.afterLegacy') }}</span>
                <button type="button" class="text-muted-foreground hover:text-foreground" :title="t('common.cancel')" @click="clearAfter"><X class="size-4" /></button>
              </div>
              <div v-else class="text-muted-foreground mt-4 flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-xs">
                <span>{{ t('auth.haveConfig') }}</span>
                <button type="button" class="text-primary hover:underline" @click="backupInput?.click()">{{ t('auth.restoreFromBackup') }}</button>
                <span aria-hidden="true">·</span>
                <button type="button" class="text-primary hover:underline" @click="after = 'legacy'">{{ t('auth.importFromV1') }}</button>
              </div>
              <input ref="backupInput" type="file" accept=".json,application/json" class="hidden" @change="onBackupFile" />
            </template>
          </CardContent>
        </template>
      </Card>

      <p class="text-muted-foreground mt-6 text-center text-xs">
        <a href="https://github.com/lonelyman0108/cfst-ddns" target="_blank" rel="noopener" class="hover:text-foreground transition-colors">GitHub</a>
      </p>
    </div>
  </div>
</template>
