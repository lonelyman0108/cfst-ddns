<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLogo from '@/components/AppLogo.vue'
import { ChevronsUpDown, CircleHelp, KeyRound, LogOut, Monitor, Moon, Rocket, Search, Sun } from '@lucide/vue'
import { systemApi } from '@/api'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
} from '@/components/ui/sidebar'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import ChangePasswordDialog from '@/components/ChangePasswordDialog.vue'
import { confirm } from '@/composables/useConfirm'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore, type ThemeMode } from '@/stores/theme'
import CommandPalette from './CommandPalette.vue'
import SidebarAutoClose from './SidebarAutoClose.vue'
import { NAV, NAV_GROUPS } from './nav'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const theme = useThemeStore()

const pwdOpen = ref(false)
const cmdOpen = ref(false)
const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform)

const activeMenu = computed(() => route.meta.menu || route.path)
const current = computed(() => NAV.find((n) => n.path === activeMenu.value))
const isSubPage = computed(() => !!current.value && route.path !== current.value.path)

const themeMode = computed({
  get: () => theme.mode,
  set: (v: string) => theme.setMode(v as ThemeMode),
})

const version = ref('')

onMounted(() => {
  auth.fetchMe().catch(() => {})
  systemApi
    .info()
    .then((i) => (version.value = i.version))
    .catch(() => {})
})

async function logout() {
  const ok = await confirm({ title: '退出登录', description: '确定要退出当前账号吗？', confirmText: '退出' })
  if (!ok) return
  auth.reset()
  router.replace({ name: 'login' })
}
</script>

<template>
  <SidebarProvider>
    <Sidebar collapsible="icon" variant="sidebar">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" as-child>
              <router-link to="/">
                <div class="flex size-8 shrink-0 items-center justify-center">
                  <AppLogo class="size-8" />
                </div>
                <div class="grid grid-cols-1 flex-1 text-left leading-tight">
                  <span class="truncate font-semibold">cfst-ddns</span>
                  <span class="text-muted-foreground truncate text-xs">优选 IP 自动解析</span>
                </div>
              </router-link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup v-for="g in NAV_GROUPS" :key="g.key">
          <SidebarGroupLabel>{{ g.label }}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              <SidebarMenuItem v-for="n in NAV.filter((x) => x.group === g.key)" :key="n.path">
                <SidebarMenuButton as-child :is-active="activeMenu === n.path" :tooltip="n.title">
                  <router-link :to="n.path">
                    <component :is="n.icon" />
                    <span>{{ n.title }}</span>
                  </router-link>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <SidebarMenuButton size="lg" class="data-[state=open]:bg-sidebar-accent">
                  <Avatar class="size-8 rounded-lg">
                    <AvatarFallback class="bg-primary/15 text-primary rounded-lg font-semibold">
                      {{ (auth.username || 'A').slice(0, 1).toUpperCase() }}
                    </AvatarFallback>
                  </Avatar>
                  <div class="grid grid-cols-1 flex-1 text-left text-sm leading-tight">
                    <span class="truncate font-medium">{{ auth.username || '管理员' }}</span>
                    <span class="text-muted-foreground truncate text-xs">管理员</span>
                  </div>
                  <ChevronsUpDown class="ml-auto size-4" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="top" align="start" class="w-(--reka-dropdown-menu-trigger-width) min-w-56">
                <DropdownMenuLabel class="flex items-center gap-2 py-1.5 font-normal">
                  <Avatar class="size-8 rounded-lg">
                    <AvatarFallback class="bg-primary/15 text-primary rounded-lg font-semibold">
                      {{ (auth.username || 'A').slice(0, 1).toUpperCase() }}
                    </AvatarFallback>
                  </Avatar>
                  <div class="grid grid-cols-1 min-w-0 flex-1 leading-tight">
                    <span class="truncate text-sm font-medium">{{ auth.username || '管理员' }}</span>
                    <span class="text-muted-foreground truncate text-xs">cfst-ddns {{ version || '' }}</span>
                  </div>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem @select="pwdOpen = true"><KeyRound />修改密码</DropdownMenuItem>
                <DropdownMenuItem @select="router.push('/welcome')"><Rocket />快速开始向导</DropdownMenuItem>
                <DropdownMenuSub>
                  <DropdownMenuSubTrigger class="gap-2">
                    <Sun v-if="theme.mode === 'light'" class="text-muted-foreground size-4" />
                    <Moon v-else-if="theme.mode === 'dark'" class="text-muted-foreground size-4" />
                    <Monitor v-else class="text-muted-foreground size-4" />
                    主题
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent>
                    <DropdownMenuRadioGroup v-model="themeMode">
                      <DropdownMenuRadioItem value="light">亮色</DropdownMenuRadioItem>
                      <DropdownMenuRadioItem value="dark">暗色</DropdownMenuRadioItem>
                      <DropdownMenuRadioItem value="system">跟随系统</DropdownMenuRadioItem>
                    </DropdownMenuRadioGroup>
                  </DropdownMenuSubContent>
                </DropdownMenuSub>
                <DropdownMenuSeparator />
                <DropdownMenuItem variant="destructive" @select="logout"><LogOut />退出登录</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>

    <SidebarInset class="min-w-0">
      <header
        class="bg-background/80 sticky top-0 z-20 flex h-14 shrink-0 items-center gap-2 border-b px-3 backdrop-blur md:px-4"
      >
        <SidebarTrigger class="-ml-1" />
        <Separator orientation="vertical" class="mr-1 data-[orientation=vertical]:h-4" />
        <Breadcrumb class="min-w-0">
          <BreadcrumbList class="flex-nowrap">
            <BreadcrumbItem class="hidden sm:inline-flex">
              <BreadcrumbLink as-child><router-link to="/">cfst-ddns</router-link></BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator class="hidden sm:block" />
            <template v-if="isSubPage && current">
              <BreadcrumbItem>
                <BreadcrumbLink as-child><router-link :to="current.path">{{ current.title }}</router-link></BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
            </template>
            <BreadcrumbItem class="min-w-0">
              <BreadcrumbPage class="truncate">{{ route.meta.title }}</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
        <div class="ml-auto flex items-center gap-1.5">
          <Button
            variant="outline"
            size="sm"
            class="text-muted-foreground hidden h-8 w-56 justify-start gap-2 font-normal md:flex"
            @click="cmdOpen = true"
          >
            <Search class="size-4" />
            <span>搜索或执行命令…</span>
            <kbd class="bg-muted ml-auto rounded border px-1.5 font-mono text-xs">{{ isMac ? '⌘' : 'Ctrl' }} K</kbd>
          </Button>
          <Button variant="ghost" size="icon" class="size-8 md:hidden" @click="cmdOpen = true"><Search /></Button>
          <Button variant="ghost" size="icon" class="size-8" as-child>
            <router-link to="/help" aria-label="帮助" title="帮助"><CircleHelp /></router-link>
          </Button>
          <Button variant="ghost" size="icon" class="size-8" :title="theme.dark ? '切换到亮色' : '切换到暗色'" @click="theme.toggle()">
            <Sun v-if="theme.dark" />
            <Moon v-else />
          </Button>
          <Button variant="ghost" size="icon" class="size-8" as-child>
            <a href="https://github.com/lonelyman0108/cfst-ddns" target="_blank" rel="noopener" aria-label="GitHub" title="GitHub">
              <svg viewBox="0 0 16 16" class="size-4" fill="currentColor" aria-hidden="true">
                <path
                  d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"
                />
              </svg>
            </a>
          </Button>
        </div>
      </header>
      <main class="mx-auto w-full max-w-[1400px] flex-1 p-4 md:p-6">
        <router-view v-slot="{ Component, route: r }">
          <component :is="Component" :key="r.path" />
        </router-view>
      </main>
    </SidebarInset>

    <CommandPalette v-model="cmdOpen" />
    <SidebarAutoClose />
    <ChangePasswordDialog v-model="pwdOpen" />
  </SidebarProvider>
</template>
