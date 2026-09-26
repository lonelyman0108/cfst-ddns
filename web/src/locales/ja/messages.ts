// 执行结果与 DNS 变更的固定说明（对应后端 messageKey）
export default {
  run: {
    canceled: "キャンセル済み",
    dryRunDone: "ドライラン完了（DNS は変更していません）",
    dnsUpdated: "DNS レコードを更新しました",
    ipUnchanged: "IP に変更はありません",
    partialFailed: "{total} 件中 {failed} 件のレコード更新に失敗しました",
    allFailed: "すべての DNS レコードの更新に失敗しました",
    noIP: "利用可能な IP が見つからず、DNS レコードは変更していません",
    noTargets: "タスクに対象レコードが設定されていません",
    interrupted: "サービスの再起動により中断されました",
  },
  change: {
    keptNoIP: "利用可能な IP がないため、既存のレコードを保持しました",
    accountUnavailable: "DNS アカウントを利用できません：{error}",
  },
}
