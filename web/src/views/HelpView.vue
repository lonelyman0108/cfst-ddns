<script setup lang="ts">
import { useRouter } from 'vue-router'
import { ArrowRight, Copy } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import PageHeader from '@/components/PageHeader.vue'
import SectionNav from '@/components/SectionNav.vue'
import { HELP_SECTIONS } from '@/components/help/sections'
import { copyText } from '@/utils/format'

const router = useRouter()

const STEPS = [
  { title: '准备 cfst', text: '首次启动会自动下载。国内网络可在向导中测速选择 GitHub 镜像，或上传本地压缩包。', to: '/cfst' },
  { title: '添加 DNS 账号', text: '填写服务商 API 凭据，测试连接通过后保存。', to: '/accounts' },
  { title: '添加通知渠道（可选）', text: '测速成功、失败或 IP 变化时推送到手机。', to: '/notifiers' },
  { title: '新建任务', text: '选择目标记录与执行周期，测速参数保持默认即可。', to: '/tasks/new' },
  { title: '执行并查看日志', text: '先「试运行」确认测速正常，再正式执行写入 DNS。', to: '/tasks' },
]

const PARAMS: { flag: string; name: string; def: string; desc: string }[] = [
  { flag: '-n', name: '线程数', def: '200', desc: '延迟测速的并发数。路由器等弱设备请调低。' },
  { flag: '-t', name: '测速次数', def: '4', desc: '每个 IP 的延迟测速次数。' },
  { flag: '-tp', name: '端口', def: '443', desc: '延迟与下载测速使用的端口。' },
  { flag: '-tl', name: '延迟上限', def: '9999 ms', desc: '只保留平均延迟低于该值的 IP。' },
  { flag: '-tll', name: '延迟下限', def: '0 ms', desc: '只保留平均延迟高于该值的 IP，可用于过滤假结果。' },
  { flag: '-tlr', name: '丢包率上限', def: '1', desc: '0–1，0 表示过滤掉任何丢包的 IP。' },
  { flag: '-dn', name: '测速数量', def: '10', desc: '按延迟排序后，取前 N 个做下载测速。' },
  { flag: '-dt', name: '测速时间', def: '10 秒', desc: '单个 IP 下载测速的最长时间。' },
  { flag: '-sl', name: '速度下限', def: '0 MB/s', desc: '只保留下载速度高于该值的 IP。建议同时设置延迟上限。' },
  { flag: '-dd', name: '禁用下载测速', def: '关', desc: '只测延迟并按延迟排序，速度更快。' },
  { flag: '-url', name: '测速地址', def: '内置', desc: '下载测速 / HTTPing 使用的地址。内置地址不保证可用，建议自建。' },
  { flag: '-httping', name: 'HTTPing 模式', def: '关', desc: '延迟测速改用 HTTP 协议（默认 TCPing）。' },
  { flag: '-httping-code', name: '有效状态码', def: '200/301/302', desc: 'HTTPing 时视为有效的状态码。' },
  { flag: '-cfcolo', name: '匹配地区', def: '—', desc: '只保留指定地区的 IP，如 HKG,NRT,LAX。仅 HTTPing 模式有效。' },
  { flag: '-allip', name: '测速全部 IP', def: '关', desc: '测速 IP 段内每个 IPv4，耗时显著增加。' },
  { flag: '-f', name: 'IP 段', def: 'ip.txt', desc: '默认使用 cfst 目录下的 ip.txt / ipv6.txt，也可为任务单独指定。' },
]

const FAQ: { q: string; a: string }[] = [
  {
    q: '为什么必须使用 host 网络？',
    a: 'Docker 的 bridge 网络会影响测速结果。Docker Desktop（Windows / macOS）不支持 host 网络，建议在 Linux 上部署，或直接运行二进制。',
  },
  {
    q: '延迟只有 1 ms 左右，结果可信吗？',
    a: '不可信。本机如果运行了 Clash、Surge 等 TUN 或透明代理，测到的延迟会异常低。请把运行测速的设备排除在代理之外。',
  },
  {
    q: '下载速度总是 0？',
    a: 'cfst 内置的测速地址不保证可用。请在任务中设置自建的「测速地址」，或开启「禁用下载测速」只按延迟排序。',
  },
  {
    q: '测速一直不结束？',
    a: '只设置了速度下限（-sl）时，如果凑不够满足条件的 IP，cfst 会一直测速。请同时设置延迟上限（-tl），或降低速度下限。',
  },
  {
    q: 'cfst 下载失败怎么办？',
    a: '在「cfst 管理」或快速开始向导中测速并选择一个可用的 GitHub 镜像；也可以从 GitHub 手动下载压缩包后上传导入。',
  },
  {
    q: '试运行和正式执行有什么区别？',
    a: '试运行只测速并记录结果，不修改 DNS、不发通知，也不计入仪表盘的当前记录与趋势。适合调整参数时使用。',
  },
  {
    q: '忘记管理员密码？',
    a: '在服务器上执行 reset-password 命令重置，见下方命令。',
  },
]

const RESET_CMD = 'docker exec cfst-ddns cfst-ddns reset-password -username admin -password 新密码'
const HOOK_CMD = `curl -X POST "${window.location.origin}/api/hooks/tasks/<任务ID>/run?token=<令牌>"`
</script>

<template>
  <div class="mx-auto flex w-full max-w-5xl flex-col gap-4">
    <PageHeader title="帮助" description="使用说明、参数解释与常见问题" />

    <SectionNav :sections="HELP_SECTIONS">
      <Card id="quick-start" class="scroll-mt-20">
        <CardHeader><CardTitle>快速上手</CardTitle></CardHeader>
        <CardContent class="grid gap-3">
          <ol class="grid gap-2">
            <li v-for="(s, i) in STEPS" :key="s.title" class="flex gap-3">
              <span class="bg-muted flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-medium tabular-nums">{{ i + 1 }}</span>
              <div class="min-w-0 flex-1">
                <router-link :to="s.to" class="text-sm font-medium hover:underline">{{ s.title }}</router-link>
                <p class="text-muted-foreground text-sm">{{ s.text }}</p>
              </div>
            </li>
          </ol>
          <div>
            <Button size="sm" @click="router.push('/welcome')">打开快速开始向导<ArrowRight /></Button>
          </div>
        </CardContent>
      </Card>

      <Card id="cfst-params" class="scroll-mt-20 gap-0 pb-0">
        <CardHeader class="pb-4">
          <CardTitle>cfst 参数说明</CardTitle>
          <p class="text-muted-foreground text-sm">任务编辑页「测速参数」中的每一项对应一个 cfst 命令行参数。表单中没有的参数可以填到「附加参数」。</p>
        </CardHeader>
        <CardContent class="overflow-x-auto border-t p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead class="pl-5">参数</TableHead>
                <TableHead>名称</TableHead>
                <TableHead>默认</TableHead>
                <TableHead class="pr-5">说明</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="p in PARAMS" :key="p.flag">
                <TableCell class="pl-5 font-mono text-code">{{ p.flag }}</TableCell>
                <TableCell class="whitespace-nowrap">{{ p.name }}</TableCell>
                <TableCell class="text-muted-foreground whitespace-nowrap">{{ p.def }}</TableCell>
                <TableCell class="text-muted-foreground min-w-64 pr-5 whitespace-normal">{{ p.desc }}</TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card id="faq" class="scroll-mt-20">
        <CardHeader><CardTitle>常见问题</CardTitle></CardHeader>
        <CardContent class="grid gap-4">
          <div v-for="f in FAQ" :key="f.q">
            <h3 class="text-sm font-medium">{{ f.q }}</h3>
            <p class="text-muted-foreground mt-1 text-sm leading-relaxed">{{ f.a }}</p>
          </div>
          <div class="bg-term text-term-foreground relative rounded-lg border border-zinc-800 p-3 pr-11 font-mono text-code break-all">
            {{ RESET_CMD }}
            <button type="button" class="absolute top-2 right-2 rounded p-1.5 text-zinc-400 hover:bg-white/10 hover:text-white" title="复制" @click="copyText(RESET_CMD)">
              <Copy class="size-3.5" />
            </button>
          </div>
          <p class="text-muted-foreground text-xs">二进制部署：<code>./cfst-ddns reset-password -data ./data -username admin -password 新密码</code></p>
        </CardContent>
      </Card>

      <Card id="webhook" class="scroll-mt-20">
        <CardHeader><CardTitle>Webhook 用法</CardTitle></CardHeader>
        <CardContent class="grid gap-3 text-sm">
          <p class="text-muted-foreground">
            在「系统设置 → Webhook 触发」中启用后，外部系统（如路由器拨号脚本、定时器、智能家居）可以不登录直接触发任务。
          </p>
          <div class="bg-term text-term-foreground relative rounded-lg border border-zinc-800 p-3 pr-11 font-mono text-code break-all">
            {{ HOOK_CMD }}
            <button type="button" class="absolute top-2 right-2 rounded p-1.5 text-zinc-400 hover:bg-white/10 hover:text-white" title="复制" @click="copyText(HOOK_CMD)">
              <Copy class="size-3.5" />
            </button>
          </div>
          <ul class="text-muted-foreground list-disc space-y-1 pl-5">
            <li>任务 ID 是任务编辑页地址中的数字。</li>
            <li>GET 与 POST 均可，成功返回 <code>{"runId": 123}</code>；任务已在运行时返回 409。</li>
            <li>令牌泄露后请在设置页重新生成，旧令牌立即失效。</li>
          </ul>
          <div>
            <Button variant="outline" size="sm" @click="router.push('/settings#webhook')">前往设置<ArrowRight /></Button>
          </div>
        </CardContent>
      </Card>

      <Card id="backup" class="scroll-mt-20">
        <CardHeader><CardTitle>备份与迁移</CardTitle></CardHeader>
        <CardContent class="grid gap-3 text-sm">
          <ul class="text-muted-foreground list-disc space-y-1 pl-5">
            <li>「系统设置 → 备份与恢复」可导出全部配置为 JSON，文件包含<strong class="text-foreground">明文凭据</strong>，请妥善保管。</li>
            <li>恢复会覆盖现有的账号、通知渠道、任务与设置。全新部署时，也可以在创建管理员的页面直接选择备份文件恢复。</li>
            <li>
              直接复制 <code>data/</code> 目录迁移时，务必带上 <code>secret.key</code>（或保持 <code>CFST_DDNS_SECRET</code> 不变），否则已保存的凭据无法解密。
            </li>
            <li>从 v1（Bash 脚本版）升级：粘贴旧的 config.sh 或环境变量，预览后一键创建账号、通知与任务。</li>
          </ul>
          <div class="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" @click="router.push('/settings#backup')">备份与恢复</Button>
            <Button variant="outline" size="sm" @click="router.push('/import/legacy')">从 v1 导入</Button>
          </div>
        </CardContent>
      </Card>
    </SectionNav>
  </div>
</template>
