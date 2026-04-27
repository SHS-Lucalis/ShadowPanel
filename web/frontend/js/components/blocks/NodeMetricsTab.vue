<template>
  <div>
    <n-alert v-if="errorMessage" type="warning" class="mb-3" :title="trans('servers.statistics_error')">
      {{ errorMessage }}
    </n-alert>

    <div v-if="!hasAnyData && !errorMessage">
      <GCard class="mb-3">
        <Loading />
        <div class="text-center text-stone-500 mt-2">
          {{ trans('servers.statistics_no_data') }}
        </div>
      </GCard>
    </div>

    <div v-if="hasAnyData" class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <GCard :title="trans('servers.statistics_cpu')" class="mb-3">
        <v-chart class="h-56 lg:h-64 w-full" :option="cpuOption" :update-options="updateOptions" autoresize />
      </GCard>

      <GCard :title="trans('servers.statistics_memory')" class="mb-3">
        <v-chart class="h-56 lg:h-64 w-full" :option="memoryOption" :update-options="updateOptions" autoresize />
      </GCard>

      <GCard :title="trans('servers.statistics_disk_io')" class="mb-3">
        <v-chart class="h-56 lg:h-64 w-full" :option="diskOption" :update-options="updateOptions" autoresize />
      </GCard>

      <GCard :title="trans('servers.statistics_network_io')" class="mb-3">
        <v-chart class="h-56 lg:h-64 w-full" :option="networkOption" :update-options="updateOptions" autoresize />
      </GCard>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import {
    GridComponent,
    TooltipComponent,
    LegendComponent,
    DataZoomComponent,
} from 'echarts/components'
import VChart from 'vue-echarts'
import { NAlert } from 'naive-ui'
import { Loading, GCard } from '@gameap/ui'
import { useNodeMetricsWebSocket } from '@/composables/useNodeMetricsWebSocket'
import { trans } from '@/i18n/i18n'

use([
    CanvasRenderer,
    LineChart,
    GridComponent,
    TooltipComponent,
    LegendComponent,
    DataZoomComponent,
])

const props = defineProps({
    nodeId: { type: [Number, String], required: true },
})

const PALETTE_LIME = '#84cc16'
const PALETTE_CYAN = '#22d3ee'
const PALETTE_MAGENTA = '#e879f9'
const PALETTE_DANGER = '#ef4444'

const updateOptions = { replaceMerge: ['series'] }

const {
    errorMessage,
    cpuSeries,
    memoryBytesSeries,
    diskReadSeries,
    diskWriteSeries,
    networkInSeries,
    networkOutSeries,
} = useNodeMetricsWebSocket(() => Number(props.nodeId))

const hasAnyData = computed(() => {
    return (
        cpuSeries.value.length > 0
        || memoryBytesSeries.value.length > 0
        || diskReadSeries.value.length > 0
        || diskWriteSeries.value.length > 0
        || networkInSeries.value.length > 0
        || networkOutSeries.value.length > 0
    )
})

function formatBytes(v) {
    if (v == null || Number.isNaN(v)) return ''
    const u = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
    let i = 0
    let n = Number(v)
    while (n >= 1024 && i < u.length - 1) {
        n /= 1024
        i++
    }
    return `${n.toFixed(1)} ${u[i]}`
}

function formatPercent(v) {
    if (v == null || Number.isNaN(v)) return ''
    return `${Number(v).toFixed(1)}%`
}

function formatBitrate(v) {
    return `${formatBytes(v)}/s`
}

function pointsToData(points) {
    return points.map(p => [p.ts, p.v])
}

function baseOption({ yMax = null, yFormatter, palette, showLegend = false }) {
    return {
        animation: false,
        grid: {
            left: 12,
            right: 16,
            top: showLegend ? 28 : 12,
            bottom: 32,
            containLabel: true,
        },
        tooltip: {
            trigger: 'axis',
            valueFormatter: yFormatter,
        },
        legend: showLegend ? { type: 'scroll', top: 0 } : { show: false },
        xAxis: {
            type: 'time',
        },
        yAxis: {
            type: 'value',
            min: 0,
            max: yMax,
            axisLabel: { formatter: yFormatter },
        },
        color: palette,
    }
}

function makeLineSeries(name, points, extra = {}) {
    return {
        name,
        type: 'line',
        showSymbol: false,
        sampling: 'lttb',
        smooth: false,
        data: pointsToData(points),
        ...extra,
    }
}

const cpuOption = computed(() => {
    const opt = baseOption({
        yMax: 100,
        yFormatter: formatPercent,
        palette: [PALETTE_LIME],
    })
    opt.series = cpuSeries.value.map(s =>
        makeLineSeries(trans('servers.statistics_cpu'), s.points, { areaStyle: { opacity: 0.12 } }),
    )
    return opt
})

const memoryOption = computed(() => {
    const opt = baseOption({
        yMax: null,
        yFormatter: formatBytes,
        palette: [PALETTE_CYAN],
    })
    opt.series = memoryBytesSeries.value.map(s =>
        makeLineSeries(trans('servers.statistics_memory'), s.points, { areaStyle: { opacity: 0.12 } }),
    )
    return opt
})

const diskOption = computed(() => {
    const opt = baseOption({
        yFormatter: formatBitrate,
        palette: [PALETTE_LIME, PALETTE_MAGENTA],
        showLegend: true,
    })
    const seriesList = []
    for (const s of diskReadSeries.value) {
        seriesList.push(makeLineSeries(trans('servers.statistics_read'), s.points))
    }
    for (const s of diskWriteSeries.value) {
        seriesList.push(makeLineSeries(trans('servers.statistics_write'), s.points))
    }
    opt.series = seriesList
    return opt
})

const networkOption = computed(() => {
    const opt = baseOption({
        yFormatter: formatBitrate,
        palette: [PALETTE_CYAN, PALETTE_DANGER],
        showLegend: true,
    })
    const seriesList = []
    for (const s of networkInSeries.value) {
        seriesList.push(makeLineSeries(trans('servers.statistics_in'), s.points))
    }
    for (const s of networkOutSeries.value) {
        seriesList.push(makeLineSeries(trans('servers.statistics_out'), s.points))
    }
    opt.series = seriesList
    return opt
})
</script>
