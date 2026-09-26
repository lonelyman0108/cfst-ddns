// 状态 / 触发方式 / DNS 操作等枚举标签，时长单位，以及通用的复制、请求错误、表单提示
export default {
  status: {
    queued: '排队中',
    running: '运行中',
    success: '成功',
    partial: '部分成功',
    failed: '失败',
    canceled: '已取消',
  },
  trigger: {
    manual: '手动',
    cron: '定时',
  },
  action: {
    create: '新建',
    update: '更新',
    delete: '删除',
    skip: '跳过',
    error: '错误',
  },
  duration: {
    s: '{s} 秒',
    ms: '{m} 分 {s} 秒',
    hm: '{h} 时 {m} 分',
  },
  copyFailed: '复制失败，请手动复制',
  http: {
    timeout: '请求超时',
    network: '网络错误，无法连接到服务器',
    status: '请求失败（{status}）',
  },
  form: {
    secretPlaceholder: '******（已保存，留空则保持不变）',
    required: '请填写{label}',
    select: '请选择',
  },
}
