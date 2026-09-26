// 执行结果与 DNS 变更的固定说明（对应后端 messageKey）
export default {
  run: {
    canceled: "已取消",
    dryRunDone: "試運行完成，未修改 DNS",
    dnsUpdated: "DNS 紀錄已更新",
    ipUnchanged: "IP 未變化",
    partialFailed: "{failed}/{total} 筆紀錄更新失敗",
    allFailed: "所有 DNS 紀錄更新失敗",
    noIP: "沒有取得任何可用 IP，DNS 紀錄未修改",
    noTargets: "任務未設定目標紀錄",
    interrupted: "服務重新啟動，執行被中斷",
  },
  change: {
    keptNoIP: "無可用 IP，保留原紀錄",
    accountUnavailable: "DNS 帳號無法使用：{error}",
  },
}
