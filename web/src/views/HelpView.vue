<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowRight, Copy } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import PageHeader from '@/components/PageHeader.vue'
import SectionNav from '@/components/SectionNav.vue'
import { HELP_SECTIONS } from '@/components/help/sections'
import { copyText } from '@/utils/format'

const router = useRouter()
const { t } = useI18n()

// 文案 key 前缀：help.steps.<key> / help.params.<key> / help.faq.<key>
const STEPS = [
  { key: 'cfst', to: '/cfst' },
  { key: 'account', to: '/accounts' },
  { key: 'notifier', to: '/notifiers' },
  { key: 'task', to: '/tasks/new' },
  { key: 'run', to: '/tasks' },
]

const PARAMS = computed<{ flag: string; name: string; def: string; desc: string }[]>(() => {
  const off = t('help.defaults.off')
  const rows: [string, string, string][] = [
    ['-n', 'n', '200'],
    ['-t', 't', '4'],
    ['-tp', 'tp', '443'],
    ['-tl', 'tl', '9999 ms'],
    ['-tll', 'tll', '0 ms'],
    ['-tlr', 'tlr', '1'],
    ['-dn', 'dn', '10'],
    ['-dt', 'dt', t('help.defaults.seconds', { n: 10 })],
    ['-sl', 'sl', '0 MB/s'],
    ['-dd', 'dd', off],
    ['-url', 'url', t('help.defaults.builtin')],
    ['-httping', 'httping', off],
    ['-httping-code', 'httpingCode', '200/301/302'],
    ['-cfcolo', 'cfcolo', '—'],
    ['-allip', 'allip', off],
    ['-f', 'f', 'ip.txt'],
  ]
  return rows.map(([flag, key, def]) => ({ flag, def, name: t(`help.params.${key}.name`), desc: t(`help.params.${key}.desc`) }))
})

const FAQ = ['hostNetwork', 'lowLatency', 'zeroSpeed', 'neverEnds', 'downloadFailed', 'dryRun', 'forgotPassword']

const sections = computed(() => HELP_SECTIONS.map((s) => ({ ...s, title: t(s.title) })))

const RESET_CMD = computed(() => `docker exec cfst-ddns cfst-ddns reset-password -username admin -password ${t('help.newPassword')}`)
const RESET_BIN = computed(() => `./cfst-ddns reset-password -data ./data -username admin -password ${t('help.newPassword')}`)
const HOOK_CMD = computed(
  () => `curl -X POST "${window.location.origin}/api/hooks/tasks/<${t('help.webhook.taskId')}>/run?token=<${t('help.webhook.token')}>"`,
)
</script>

<template>
  <div class="mx-auto flex w-full max-w-5xl flex-col gap-4">
    <PageHeader :title="t('nav.help')" :description="t('help.description')" />

    <SectionNav :sections="sections">
      <Card id="quick-start" class="scroll-mt-20">
        <CardHeader><CardTitle>{{ t('help.sections.quickStart') }}</CardTitle></CardHeader>
        <CardContent class="grid grid-cols-1 gap-3">
          <ol class="grid grid-cols-1 gap-2">
            <li v-for="(s, i) in STEPS" :key="s.key" class="flex gap-3">
              <span class="bg-muted flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-medium tabular-nums">{{ i + 1 }}</span>
              <div class="min-w-0 flex-1">
                <router-link :to="s.to" class="text-sm font-medium hover:underline">{{ t(`help.steps.${s.key}.title`) }}</router-link>
                <p class="text-muted-foreground text-sm">{{ t(`help.steps.${s.key}.text`) }}</p>
              </div>
            </li>
          </ol>
          <div>
            <Button size="sm" @click="router.push('/welcome')">{{ t('help.openWizard') }}<ArrowRight /></Button>
          </div>
        </CardContent>
      </Card>

      <Card id="cfst-params" class="scroll-mt-20 gap-0 pb-0">
        <CardHeader class="pb-4">
          <CardTitle>{{ t('help.sections.cfstParams') }}</CardTitle>
          <p class="text-muted-foreground text-sm">{{ t('help.paramsIntro') }}</p>
        </CardHeader>
        <CardContent class="overflow-x-auto border-t p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead class="pl-5">{{ t('help.paramCols.flag') }}</TableHead>
                <TableHead>{{ t('help.paramCols.name') }}</TableHead>
                <TableHead>{{ t('help.paramCols.def') }}</TableHead>
                <TableHead class="pr-5">{{ t('help.paramCols.desc') }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="p in PARAMS" :key="p.flag">
                <TableCell class="pl-5 font-mono text-code">{{ p.flag }}</TableCell>
                <TableCell class="min-w-20 whitespace-normal">{{ p.name }}</TableCell>
                <TableCell class="text-muted-foreground min-w-16 whitespace-normal">{{ p.def }}</TableCell>
                <TableCell class="text-muted-foreground min-w-48 pr-5 whitespace-normal">{{ p.desc }}</TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card id="faq" class="scroll-mt-20">
        <CardHeader><CardTitle>{{ t('help.sections.faq') }}</CardTitle></CardHeader>
        <CardContent class="grid grid-cols-1 gap-4">
          <div v-for="f in FAQ" :key="f">
            <h3 class="text-sm font-medium">{{ t(`help.faq.${f}.q`) }}</h3>
            <p class="text-muted-foreground mt-1 text-sm leading-relaxed">{{ t(`help.faq.${f}.a`) }}</p>
          </div>
          <div class="bg-term text-term-foreground relative rounded-lg border border-zinc-800 p-3 pr-11 font-mono text-code break-all">
            {{ RESET_CMD }}
            <button type="button" class="absolute top-2 right-2 rounded p-1.5 text-zinc-400 hover:bg-white/10 hover:text-white" :title="t('common.copy')" @click="copyText(RESET_CMD)">
              <Copy class="size-3.5" />
            </button>
          </div>
          <i18n-t keypath="help.resetBinary" tag="p" class="text-muted-foreground text-xs">
            <template #cmd><code>{{ RESET_BIN }}</code></template>
          </i18n-t>
        </CardContent>
      </Card>

      <Card id="webhook" class="scroll-mt-20">
        <CardHeader><CardTitle>{{ t('help.sections.webhook') }}</CardTitle></CardHeader>
        <CardContent class="grid grid-cols-1 gap-3 text-sm">
          <p class="text-muted-foreground">
            {{ t('help.webhook.intro') }}
          </p>
          <div class="bg-term text-term-foreground relative rounded-lg border border-zinc-800 p-3 pr-11 font-mono text-code break-all">
            {{ HOOK_CMD }}
            <button type="button" class="absolute top-2 right-2 rounded p-1.5 text-zinc-400 hover:bg-white/10 hover:text-white" :title="t('common.copy')" @click="copyText(HOOK_CMD)">
              <Copy class="size-3.5" />
            </button>
          </div>
          <ul class="text-muted-foreground list-disc space-y-1 pl-5">
            <li>{{ t('help.webhook.tipTaskId') }}</li>
            <i18n-t keypath="help.webhook.tipResponse" tag="li">
              <template #json><code>{"runId": 123}</code></template>
            </i18n-t>
            <li>{{ t('help.webhook.tipToken') }}</li>
          </ul>
          <div>
            <Button variant="outline" size="sm" @click="router.push('/settings#webhook')">{{ t('help.webhook.goSettings') }}<ArrowRight /></Button>
          </div>
        </CardContent>
      </Card>

      <Card id="backup" class="scroll-mt-20">
        <CardHeader><CardTitle>{{ t('help.sections.backup') }}</CardTitle></CardHeader>
        <CardContent class="grid grid-cols-1 gap-3 text-sm">
          <ul class="text-muted-foreground list-disc space-y-1 pl-5">
            <i18n-t keypath="help.backup.export" tag="li">
              <template #secret><strong class="text-foreground">{{ t('help.backup.plaintext') }}</strong></template>
            </i18n-t>
            <li>{{ t('help.backup.restore') }}</li>
            <i18n-t keypath="help.backup.copyData" tag="li">
              <template #dir><code>data/</code></template>
              <template #key><code>secret.key</code></template>
              <template #env><code>CFST_DDNS_SECRET</code></template>
            </i18n-t>
            <li>{{ t('help.backup.legacy') }}</li>
          </ul>
          <div class="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" @click="router.push('/settings#backup')">{{ t('help.backup.backupRestore') }}</Button>
            <Button variant="outline" size="sm" @click="router.push('/import/legacy')">{{ t('help.backup.importV1') }}</Button>
          </div>
        </CardContent>
      </Card>
    </SectionNav>
  </div>
</template>
