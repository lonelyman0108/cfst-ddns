// 状态 / 触发方式 / DNS 操作等枚举标签，时长单位，以及通用的复制、请求错误、表单提示
export default {
  status: {
    queued: '待機中',
    running: '実行中',
    success: '成功',
    partial: '一部成功',
    failed: '失敗',
    canceled: 'キャンセル済み',
  },
  trigger: {
    manual: '手動',
    cron: '定期',
  },
  action: {
    create: '作成',
    update: '更新',
    delete: '削除',
    skip: 'スキップ',
    error: 'エラー',
  },
  duration: {
    s: '{s} 秒',
    ms: '{m} 分 {s} 秒',
    hm: '{h} 時間 {m} 分',
  },
  copyFailed: 'コピーに失敗しました。手動でコピーしてください',
  http: {
    timeout: 'リクエストがタイムアウトしました',
    network: 'ネットワークエラー：サーバーに接続できません',
    status: 'リクエストに失敗しました（{status}）',
  },
  form: {
    secretPlaceholder: '******（保存済み。空欄のままなら変更しません）',
    required: '{label}を入力してください',
    select: '選択してください',
  },
}
