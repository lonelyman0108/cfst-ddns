<script setup lang="ts">
import AppLogo from '@/components/AppLogo.vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { Loader2, Moon, RefreshCw, ServerCrash, Sun } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import FormItem from '@/components/FormItem.vue'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'

const auth = useAuthStore()
const theme = useThemeStore()
const route = useRoute()
const router = useRouter()

const loading = ref(false)
const statusLoading = ref(true)
const statusError = ref(false)
const form = reactive({ username: '', password: '', confirm: '' })
const errors = reactive<Record<string, string>>({})

const isSetup = computed(() => auth.initialized === false)

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
  if (!u) errors.username = '请输入用户名'
  else if (isSetup.value && (u.length < 3 || u.length > 32)) errors.username = '用户名长度为 3–32 位'
  if (!form.password) errors.password = '请输入密码'
  else if (isSetup.value && form.password.length < 6) errors.password = '密码至少 6 位'
  if (isSetup.value && form.confirm !== form.password) errors.confirm = '两次输入的密码不一致'
  return !Object.keys(errors).length
}

async function submit() {
  if (!validate()) return
  loading.value = true
  try {
    const cred = { username: form.username.trim(), password: form.password }
    if (isSetup.value) {
      await auth.setup(cred)
      toast.success('管理员已创建')
    } else {
      await auth.login(cred)
    }
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

    <Button variant="ghost" size="icon" class="absolute top-4 right-4" @click="theme.toggle()">
      <Sun v-if="theme.dark" />
      <Moon v-else />
    </Button>

    <div class="relative z-10 w-full max-w-sm">
      <div class="mb-8 flex flex-col items-center text-center">
        <AppLogo class="shadow-primary/25 mb-4 size-14 rounded-[13px] shadow-lg" />
        <h1 class="text-2xl font-semibold tracking-[-0.01em]">cfst-ddns</h1>
        <p class="text-muted-foreground mt-1.5 text-sm">测速优选 Cloudflare IP，自动写入你的 DNS 记录</p>
      </div>

      <Card class="shadow-lg shadow-black/5">
        <template v-if="statusLoading">
          <CardHeader><Skeleton class="h-5 w-24" /><Skeleton class="h-4 w-48" /></CardHeader>
          <CardContent class="grid gap-4"><Skeleton class="h-9" /><Skeleton class="h-9" /><Skeleton class="h-9" /></CardContent>
        </template>
        <template v-else-if="statusError">
          <CardContent class="flex flex-col items-center gap-3 py-6 text-center">
            <ServerCrash class="text-muted-foreground size-10" />
            <div class="font-medium">无法连接到服务器</div>
            <p class="text-muted-foreground text-sm">请确认后端服务已启动</p>
            <Button variant="outline" size="sm" @click="loadStatus"><RefreshCw />重试</Button>
          </CardContent>
        </template>
        <template v-else>
          <CardHeader>
            <CardTitle>{{ isSetup ? '创建管理员' : '登录' }}</CardTitle>
            <CardDescription>{{ isSetup ? '首次使用，请设置管理员账号与密码' : '使用管理员账号登录控制台' }}</CardDescription>
          </CardHeader>
          <CardContent>
            <form class="grid gap-4" @submit.prevent="submit">
              <FormItem label="用户名" for="username" :error="errors.username">
                <Input id="username" v-model="form.username" autocomplete="username" autofocus :aria-invalid="!!errors.username" />
              </FormItem>
              <FormItem label="密码" for="password" :error="errors.password">
                <Input
                  id="password"
                  v-model="form.password"
                  type="password"
                  :autocomplete="isSetup ? 'new-password' : 'current-password'"
                  :aria-invalid="!!errors.password"
                />
              </FormItem>
              <FormItem v-if="isSetup" label="确认密码" for="confirm" :error="errors.confirm">
                <Input id="confirm" v-model="form.confirm" type="password" autocomplete="new-password" :aria-invalid="!!errors.confirm" />
              </FormItem>
              <Button type="submit" class="mt-1 w-full" :disabled="loading">
                <Loader2 v-if="loading" class="animate-spin" />
                {{ isSetup ? '创建并登录' : '登录' }}
              </Button>
            </form>
          </CardContent>
        </template>
      </Card>

      <p class="text-muted-foreground mt-6 text-center text-xs">
        <a href="https://github.com/lonelyman0108/cfst-ddns" target="_blank" rel="noopener" class="hover:text-foreground transition-colors">GitHub</a>
      </p>
    </div>
  </div>
</template>
