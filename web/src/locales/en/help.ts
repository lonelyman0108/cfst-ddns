// 帮助页：快速上手、cfst 参数、常见问题、Webhook、备份与迁移
export default {
  description: 'How-to guide, parameter reference and FAQ',
  sections: {
    quickStart: 'Getting started',
    cfstParams: 'cfst parameters',
    faq: 'FAQ',
    webhook: 'Using the Webhook',
    backup: 'Backup & migration',
  },
  steps: {
    cfst: {
      title: 'Set up cfst',
      text: 'It is downloaded automatically on first launch. If GitHub is slow or blocked on your network, test and pick a GitHub mirror in the wizard, or upload a local archive.',
    },
    account: { title: 'Add a DNS account', text: "Enter your provider's API credentials and save once the connection test passes." },
    notifier: { title: 'Add a notification channel (optional)', text: 'Get pushed to your phone when a speed test succeeds, fails or the IP changes.' },
    task: { title: 'Create a task', text: 'Choose the target records and schedule; the default speed-test parameters are fine to start with.' },
    run: { title: 'Run it and check the log', text: 'Start with a "Dry run" to confirm the speed test works, then run for real to write to DNS.' },
  },
  openWizard: 'Open the quick start wizard',
  paramsIntro:
    'Each field under "Speed test parameters" on the task edit page maps to a cfst command-line flag. Flags not in the form can go into "Extra arguments".',
  paramCols: { flag: 'Flag', name: 'Name', def: 'Default', desc: 'Description' },
  defaults: { off: 'Off', builtin: 'Built-in', seconds: '{n} s' },
  params: {
    n: { name: 'Threads', desc: 'Concurrency of the latency test. Lower it on weak devices such as routers.' },
    t: { name: 'Test count', desc: 'Number of latency tests per IP.' },
    tp: { name: 'Port', desc: 'Port used for latency and download tests.' },
    tl: { name: 'Max latency', desc: 'Keep only IPs whose average latency is below this value.' },
    tll: { name: 'Min latency', desc: 'Keep only IPs whose average latency is above this value; useful for filtering out bogus results.' },
    tlr: { name: 'Max packet loss', desc: '0–1; 0 filters out any IP with packet loss.' },
    dn: { name: 'Download test count', desc: 'After sorting by latency, download-test the top N IPs.' },
    dt: { name: 'Download test time', desc: 'Maximum download test time per IP.' },
    sl: { name: 'Min speed', desc: 'Keep only IPs whose download speed is above this value. Set a max latency as well.' },
    dd: { name: 'Disable download test', desc: 'Test latency only and sort by it; much faster.' },
    url: { name: 'Test URL', desc: 'URL used for download tests / HTTPing. The built-in URL is not guaranteed to work; hosting your own is recommended.' },
    httping: { name: 'HTTPing mode', desc: 'Use HTTP for the latency test (TCPing by default).' },
    httpingCode: { name: 'Valid status codes', desc: 'Status codes treated as valid in HTTPing mode.' },
    cfcolo: { name: 'Colo filter', desc: 'Keep only IPs in the given data centers, e.g. HKG,NRT,LAX. HTTPing mode only.' },
    allip: { name: 'Test all IPs', desc: 'Test every IPv4 address in the IP ranges; takes much longer.' },
    f: { name: 'IP ranges', desc: 'Defaults to ip.txt / ipv6.txt in the cfst directory; can also be set per task.' },
  },
  faq: {
    hostNetwork: {
      q: 'Why is host networking required?',
      a: "Docker's bridge network skews speed-test results. Docker Desktop (Windows / macOS) does not support host networking, so deploy on Linux or run the binary directly.",
    },
    lowLatency: {
      q: 'Latency is only about 1 ms. Can I trust that?',
      a: 'No. If the machine runs a TUN or transparent proxy such as Clash or Surge, measured latency will be abnormally low. Exclude the device running the speed test from the proxy.',
    },
    zeroSpeed: {
      q: 'Download speed is always 0?',
      a: 'The test URL built into cfst is not guaranteed to work. Set your own "Test URL" in the task, or enable "Disable download test" to sort by latency only.',
    },
    neverEnds: {
      q: 'The speed test never finishes?',
      a: 'If only a minimum speed (-sl) is set and not enough IPs meet it, cfst keeps testing. Also set a max latency (-tl), or lower the minimum speed.',
    },
    downloadFailed: {
      q: 'What if downloading cfst fails?',
      a: 'Test and pick a working GitHub mirror in "Manage cfst" or the quick start wizard, or download the archive from GitHub manually and upload it.',
    },
    dryRun: {
      q: 'What is the difference between a dry run and a real run?',
      a: "A dry run only runs the speed test and records the results. It doesn't change DNS, send notifications, or count toward the dashboard's current records and trends. Use it while tuning parameters.",
    },
    forgotPassword: {
      q: 'Forgot the administrator password?',
      a: 'Reset it by running the reset-password command on the server, as shown below.',
    },
  },
  newPassword: 'NEW_PASSWORD',
  resetBinary: 'Binary deployment: {cmd}',
  webhook: {
    intro:
      'Once enabled under "Settings → Webhook trigger", external systems (router dial-up scripts, timers, smart-home hubs, etc.) can trigger tasks without signing in.',
    taskId: 'TASK_ID',
    token: 'TOKEN',
    tipTaskId: 'The task ID is the number in the task edit page URL.',
    tipResponse: 'Both GET and POST work. On success it returns {json}; if the task is already running it returns 409.',
    tipToken: 'If the token leaks, regenerate it on the settings page; the old token stops working immediately.',
    goSettings: 'Go to settings',
  },
  backup: {
    export: '"Settings → Backup & restore" exports the whole configuration as JSON. The file contains {secret}, so keep it safe.',
    plaintext: 'plaintext credentials',
    restore:
      'Restoring overwrites existing accounts, notification channels, tasks and settings. On a fresh install you can also pick a backup file right on the create-administrator page.',
    copyData:
      'When migrating by copying the {dir} directory, be sure to include {key} (or keep {env} unchanged); otherwise saved credentials cannot be decrypted.',
    legacy: 'Upgrading from v1 (the Bash script version): paste your old config.sh or environment variables, preview, then create accounts, notifications and tasks in one click.',
    backupRestore: 'Backup & restore',
    importV1: 'Import from v1',
  },
}
