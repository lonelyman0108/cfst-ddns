// 执行结果与 DNS 变更的固定说明（对应后端 messageKey）
export default {
  run: {
    canceled: "已取消",
    dryRunDone: "试运行完成，未修改 DNS",
    dnsUpdated: "DNS 记录已更新",
    ipUnchanged: "IP 未变化",
    partialFailed: "{failed}/{total} 条记录更新失败",
    allFailed: "全部 DNS 记录更新失败",
    noIP: "没有获得任何可用 IP，DNS 记录未修改",
    noTargets: "任务未配置目标记录",
    interrupted: "服务重启，执行被中断",
  },
  change: {
    keptNoIP: "无可用 IP，保留原记录",
    accountUnavailable: "DNS 账号不可用：{error}",
  },
}
