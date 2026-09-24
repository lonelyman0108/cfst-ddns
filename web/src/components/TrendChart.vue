<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { use, graphic } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import type { TrendPoint } from '@/api/types'
import { useThemeStore } from '@/stores/theme'
import { dayjs } from '@/utils/format'

use([CanvasRenderer, LineChart, GridComponent, LegendComponent, TooltipComponent])

const props = defineProps<{ data: TrendPoint[]; metric: 'latency' | 'speed' }>()
const theme = useThemeStore()

interface Palette {
  series: string[]
  fg: string
  muted: string
  border: string
  popover: string
}

/** 读取 CSS 变量并转换为 ECharts 可解析的 rgb 颜色（变量为 oklch） */
function resolveVar(name: string): string {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  if (!raw) return '#888'
  const c = document.createElement('canvas')
  c.width = c.height = 1
  const ctx = c.getContext('2d', { willReadFrequently: true })
  if (!ctx) return raw
  ctx.fillStyle = '#000'
  ctx.fillStyle = raw
  ctx.fillRect(0, 0, 1, 1)
  const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data
  return a < 255 ? `rgba(${r},${g},${b},${(a / 255).toFixed(3)})` : `rgb(${r},${g},${b})`
}

function withAlpha(rgb: string, a: number) {
  const m = /rgba?\((\d+),(\d+),(\d+)/.exec(rgb)
  return m ? `rgba(${m[1]},${m[2]},${m[3]},${a})` : rgb
}

const palette = ref<Palette | null>(null)

function readPalette() {
  palette.value = {
    series: ['--chart-1', '--chart-2', '--chart-3', '--chart-4', '--chart-5'].map(resolveVar),
    fg: resolveVar('--foreground'),
    muted: resolveVar('--muted-foreground'),
    border: resolveVar('--border'),
    popover: resolveVar('--popover'),
  }
}

// 主题切换后 CSS 变量变化，重新取色
watch(
  () => theme.dark,
  () => nextTick(readPalette),
  { immediate: true },
)

const option = computed(() => {
  const p = palette.value
  if (!p) return {}
  const groups = new Map<string, TrendPoint[]>()
  const sorted = [...props.data].sort((a, b) => dayjs(a.time).valueOf() - dayjs(b.time).valueOf())
  const multiType = new Set(sorted.map((x) => x.ipType)).size > 1
  for (const x of sorted) {
    const key = multiType ? `${x.taskName} · ${x.ipType === 'v6' ? 'IPv6' : 'IPv4'}` : x.taskName
    if (!groups.has(key)) groups.set(key, [])
    groups.get(key)!.push(x)
  }
  const unit = props.metric === 'latency' ? 'ms' : 'MB/s'
  const series = [...groups.entries()].map(([name, pts], i) => {
    const color = p.series[i % p.series.length]
    return {
      name,
      type: 'line',
      smooth: 0.3,
      showSymbol: pts.length < 16,
      symbol: 'circle',
      symbolSize: 5,
      lineStyle: { width: 2, color },
      itemStyle: { color },
      areaStyle: {
        color: new graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: withAlpha(color, 0.28) },
          { offset: 1, color: withAlpha(color, 0.02) },
        ]),
      },
      emphasis: { focus: 'series' },
      data: pts.map((x) => {
        const v = props.metric === 'latency' ? x.latency : x.speed
        return [x.time, v > 0 ? +v.toFixed(2) : null]
      }),
    }
  })
  return {
    backgroundColor: 'transparent',
    textStyle: { fontFamily: getComputedStyle(document.body).fontFamily },
    tooltip: {
      trigger: 'axis',
      backgroundColor: p.popover,
      borderColor: p.border,
      textStyle: { color: p.fg, fontSize: 12 },
      axisPointer: { type: 'line', lineStyle: { color: p.border } },
      valueFormatter: (v: unknown) => (v === null || v === undefined ? '-' : `${v} ${unit}`),
    },
    legend: {
      type: 'scroll',
      top: 0,
      right: 0,
      icon: 'roundRect',
      itemWidth: 10,
      itemHeight: 10,
      textStyle: { color: p.muted, fontSize: 12 },
      pageTextStyle: { color: p.muted },
    },
    grid: { left: 4, right: 8, top: 36, bottom: 4, containLabel: true },
    xAxis: {
      type: 'time',
      axisLine: { lineStyle: { color: p.border } },
      axisTick: { show: false },
      splitLine: { show: false },
      axisLabel: { color: p.muted, fontSize: 11, hideOverlap: true, formatter: (v: number) => dayjs(v).format('MM-DD HH:mm') },
    },
    yAxis: {
      type: 'value',
      scale: true,
      name: unit,
      nameTextStyle: { color: p.muted, fontSize: 11, align: 'left' },
      axisLabel: { color: p.muted, fontSize: 11 },
      splitLine: { lineStyle: { color: p.border, type: 'dashed' } },
    },
    series,
  }
})
</script>

<template>
  <!-- vue-echarts 注入的非分层样式会覆盖 Tailwind 工具类，高度由外层容器决定 -->
  <div class="h-72 w-full">
    <v-chart :option="option" autoresize :update-options="{ notMerge: true }" />
  </div>
</template>
