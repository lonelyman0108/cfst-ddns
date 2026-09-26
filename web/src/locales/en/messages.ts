// 执行结果与 DNS 变更的固定说明（对应后端 messageKey）
export default {
  run: {
    canceled: "Canceled",
    dryRunDone: "Dry run finished, DNS not changed",
    dnsUpdated: "DNS records updated",
    ipUnchanged: "IP unchanged",
    partialFailed: "{failed} of {total} records failed to update",
    allFailed: "All DNS record updates failed",
    noIP: "No usable IP found, DNS records left unchanged",
    noTargets: "The task has no target records",
    interrupted: "Interrupted by a service restart",
  },
  change: {
    keptNoIP: "No usable IP, existing record kept",
    accountUnavailable: "DNS account unavailable: {error}",
  },
}
