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
    proxyDetected: "偵測到測速流量經過代理，DNS 記錄未修改",
    healthSnapshot: "健康檢查觸發的執行中，DNS 目標已變更或無法安全更新，紀錄未修改",
  },
  change: {
    keptNoIP: "無可用 IP，保留原紀錄",
    healthTargetChanged: "健康檢查目標已變更或已跳過，保留原紀錄",
  },
}
