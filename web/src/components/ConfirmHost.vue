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
import { useI18n } from 'vue-i18n'
import { confirmState, settleConfirm } from '@/composables/useConfirm'

const { t } = useI18n()

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
        <AlertDialogCancel @click="settleConfirm(false)">{{ confirmState.cancelText || t('common.cancel') }}</AlertDialogCancel>
        <Button :variant="confirmState.destructive ? 'destructive' : 'default'" @click="settleConfirm(true)">
          {{ confirmState.confirmText || t('common.confirm') }}
        </Button>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
