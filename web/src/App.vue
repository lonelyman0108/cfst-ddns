<script setup lang="ts">
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import ConfirmHost from '@/components/ConfirmHost.vue'
import { useThemeStore } from '@/stores/theme'

const theme = useThemeStore()
</script>

<template>
  <TooltipProvider :delay-duration="200">
    <router-view />
  </TooltipProvider>
  <Toaster :theme="theme.dark ? 'dark' : 'light'" position="top-center" rich-colors close-button />
  <ConfirmHost />
</template>
