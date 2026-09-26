// 状态 / 触发方式 / DNS 操作等枚举标签，时长单位，以及通用的复制、请求错误、表单提示
export default {
  status: {
    queued: 'Queued',
    running: 'Running',
    success: 'Success',
    partial: 'Partial',
    failed: 'Failed',
    canceled: 'Canceled',
  },
  trigger: {
    manual: 'Manual',
    cron: 'Scheduled',
  },
  action: {
    create: 'Create',
    update: 'Update',
    delete: 'Delete',
    skip: 'Skip',
    error: 'Error',
  },
  duration: {
    s: '{s}s',
    ms: '{m}m {s}s',
    hm: '{h}h {m}m',
  },
  copyFailed: 'Copy failed, please copy manually',
  http: {
    timeout: 'Request timed out',
    network: 'Network error: unable to reach the server',
    status: 'Request failed ({status})',
  },
  form: {
    secretPlaceholder: '****** (saved; leave blank to keep unchanged)',
    required: '{label} is required',
    select: 'Select…',
  },
}
