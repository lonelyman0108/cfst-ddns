import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { onUnauthorized } from '@/api/http'
import { useAuthStore } from '@/stores/auth'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    public?: boolean
    /** 侧栏高亮的菜单路径 */
    menu?: string
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { title: '登录', public: true },
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    children: [
      { path: '', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { title: '仪表盘', menu: '/' } },
      { path: 'tasks', name: 'tasks', component: () => import('@/views/TaskListView.vue'), meta: { title: '任务', menu: '/tasks' } },
      { path: 'tasks/new', name: 'task-new', component: () => import('@/views/TaskEditView.vue'), meta: { title: '新建任务', menu: '/tasks' } },
      { path: 'tasks/:id(\\d+)', name: 'task-edit', component: () => import('@/views/TaskEditView.vue'), meta: { title: '编辑任务', menu: '/tasks' } },
      { path: 'runs', name: 'runs', component: () => import('@/views/RunListView.vue'), meta: { title: '执行历史', menu: '/runs' } },
      { path: 'runs/:id(\\d+)', name: 'run-detail', component: () => import('@/views/RunDetailView.vue'), meta: { title: '执行详情', menu: '/runs' } },
      { path: 'accounts', name: 'accounts', component: () => import('@/views/AccountsView.vue'), meta: { title: 'DNS 账号', menu: '/accounts' } },
      { path: 'notifiers', name: 'notifiers', component: () => import('@/views/NotifiersView.vue'), meta: { title: '通知渠道', menu: '/notifiers' } },
      { path: 'cfst', name: 'cfst', component: () => import('@/views/CfstView.vue'), meta: { title: 'cfst 管理', menu: '/cfst' } },
      { path: 'settings', name: 'settings', component: () => import('@/views/SettingsView.vue'), meta: { title: '系统设置', menu: '/settings' } },
      { path: 'logs', name: 'logs', component: () => import('@/views/LogsView.vue'), meta: { title: '系统日志', menu: '/logs' } },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory('/'),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.public) {
    if (to.name === 'login' && auth.loggedIn) return { path: '/' }
    return true
  }
  if (!auth.loggedIn) {
    return { name: 'login', query: to.fullPath !== '/' ? { redirect: to.fullPath } : {} }
  }
  return true
})

router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} - cfst-ddns` : 'cfst-ddns'
})

// 401：清除登录状态并跳转登录页
onUnauthorized(() => {
  const auth = useAuthStore()
  auth.reset()
  const cur = router.currentRoute.value
  if (cur.name !== 'login') {
    router.replace({ name: 'login', query: cur.fullPath !== '/' ? { redirect: cur.fullPath } : {} })
  }
})

export default router
