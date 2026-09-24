<script setup lang="ts">
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { confirmState, settleConfirm } from '@/composables/useConfirm'

function onOpenChange(v: boolean) {
  if (!v) settleConfirm(false)
}
</script>

<template>
  <AlertDialog :open="confirmState.open" @update:open="onOpenChange">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ confirmState.title }}</AlertDialogTitle>
        <AlertDialogDescription v-if="confirmState.description" class="whitespace-pre-line">
          {{ confirmState.description }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel @click="settleConfirm(false)">{{ confirmState.cancelText || '取消' }}</AlertDialogCancel>
        <Button :variant="confirmState.destructive ? 'destructive' : 'default'" @click="settleConfirm(true)">
          {{ confirmState.confirmText || '确定' }}
        </Button>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
