<script setup lang="ts">
import { watch } from 'vue'
import { useRoute } from 'vue-router'
import { useSidebar } from '@/components/ui/sidebar'

// 移动端路由切换后自动收起侧栏抽屉（需位于 SidebarProvider 内）
const route = useRoute()
const { setOpenMobile } = useSidebar()
watch(
  () => route.fullPath,
  () => setOpenMobile(false),
)
</script>

<template>
  <span class="hidden" />
</template>
