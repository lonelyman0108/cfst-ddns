// 状态 / 触发方式 / DNS 操作等枚举标签，时长单位，以及通用的复制、请求错误、表单提示
export default {
  status: {
    queued: '佇列中',
    running: '執行中',
    success: '成功',
    partial: '部分成功',
    failed: '失敗',
    canceled: '已取消',
  },
  trigger: {
    manual: '手動',
    cron: '排程',
  },
  action: {
    create: '新增',
    update: '更新',
    delete: '刪除',
    skip: '略過',
    error: '錯誤',
  },
  duration: {
    s: '{s} 秒',
    ms: '{m} 分 {s} 秒',
    hm: '{h} 時 {m} 分',
  },
  copyFailed: '複製失敗，請手動複製',
  http: {
    timeout: '請求逾時',
    network: '網路錯誤，無法連線到伺服器',
    status: '請求失敗（{status}）',
  },
  form: {
    secretPlaceholder: '******（已儲存，留空則保持不變）',
    required: '請填寫{label}',
    select: '請選擇',
  },
}
