import { reactive } from 'vue'

export interface ConfirmOptions {
  title: string
  description?: string
  confirmText?: string
  cancelText?: string
  destructive?: boolean
}

interface ConfirmState extends ConfirmOptions {
  open: boolean
  resolve: ((v: boolean) => void) | null
}

/** 全局确认对话框状态，由 ConfirmHost 渲染 */
export const confirmState = reactive<ConfirmState>({
  open: false,
  title: '',
  resolve: null,
})

/** 打开确认框，确认返回 true，取消返回 false */
export function confirm(opts: ConfirmOptions): Promise<boolean> {
  // 若已有未决的确认框，视为取消
  confirmState.resolve?.(false)
  return new Promise((resolve) => {
    Object.assign(confirmState, {
      description: undefined,
      confirmText: undefined,
      cancelText: undefined,
      destructive: false,
      ...opts,
      open: true,
      resolve,
    })
  })
}

export function settleConfirm(v: boolean) {
  const r = confirmState.resolve
  confirmState.resolve = null
  confirmState.open = false
  r?.(v)
}
