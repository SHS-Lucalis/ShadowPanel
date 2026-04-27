import { ref, shallowRef, computed, watch, toValue } from 'vue'
import { useWebSocket } from './useWebSocket'

const WINDOW_MS = 30 * 60 * 1000
const MAX_POINTS_PER_SERIES = 600
const DEDUP_REBUILD_EVERY = 30

function seriesKey(name, labels) {
    if (!labels) return name
    const keys = Object.keys(labels).sort()
    if (keys.length === 0) return name
    const parts = keys.map(k => `${k}=${labels[k]}`)
    return `${name}|${parts.join(',')}`
}

export function useNodeMetricsWebSocket(nodeId) {
    const seriesMap = shallowRef(new Map())
    const replayDone = ref(false)
    const errorMessage = ref('')

    let dedupSet = new Set()
    const counterPrev = new Map()
    let envelopeCounter = 0

    function ingestSample(name, type, unit, labels, point) {
        const tsMs = Date.parse(point.timestamp)
        if (Number.isNaN(tsMs)) return

        let value = parseFloat(point.value)
        if (Number.isNaN(value)) return

        const key = seriesKey(name, labels)
        const dedupKey = `${key}|${tsMs}`
        if (dedupSet.has(dedupKey)) return
        dedupSet.add(dedupKey)

        if (type === 'counter') {
            const prev = counterPrev.get(key)
            counterPrev.set(key, { ts: tsMs, v: value })
            if (!prev) return
            const dt = (tsMs - prev.ts) / 1000
            const dv = value - prev.v
            if (dv < 0 || dt <= 0) return
            value = dv / dt
        }

        const map = seriesMap.value
        let series = map.get(key)
        if (!series) {
            series = { name, type, unit, labels: { ...labels }, points: [] }
            map.set(key, series)
        }
        series.points.push({ ts: tsMs, v: value })

        const cutoff = Date.now() - WINDOW_MS
        while (series.points.length > 0 && series.points[0].ts < cutoff) {
            series.points.shift()
        }
        if (series.points.length > MAX_POINTS_PER_SERIES) {
            series.points.splice(0, series.points.length - MAX_POINTS_PER_SERIES)
        }
    }

    function ingestEnvelope(env) {
        if (!env || !Array.isArray(env.series)) return
        for (const s of env.series) {
            if (!s || !Array.isArray(s.points)) continue
            for (const p of s.points) {
                ingestSample(s.name, s.type, s.unit, s.labels || {}, p)
            }
        }
        envelopeCounter++
        if (envelopeCounter >= DEDUP_REBUILD_EVERY) {
            envelopeCounter = 0
            const fresh = new Set()
            for (const series of seriesMap.value.values()) {
                const key = seriesKey(series.name, series.labels)
                for (const pt of series.points) {
                    fresh.add(`${key}|${pt.ts}`)
                }
            }
            dedupSet = fresh
        }
        seriesMap.value = new Map(seriesMap.value)
    }

    const ws = useWebSocket({
        onMessage(msg) {
            if (msg.type === 'metrics.replay') {
                const list = Array.isArray(msg.payload) ? msg.payload : []
                for (const env of list) {
                    ingestEnvelope(env)
                }
            } else if (msg.type === 'metrics.replay.done') {
                replayDone.value = true
            } else if (msg.type === 'metrics') {
                ingestEnvelope(msg.payload)
            } else if (msg.type === 'metrics.error') {
                errorMessage.value = msg.payload?.error || 'unknown error'
            }
        },
        onOpen() {
            errorMessage.value = ''
        },
    })

    watch(
        () => toValue(nodeId),
        (id) => {
            seriesMap.value = new Map()
            replayDone.value = false
            dedupSet = new Set()
            counterPrev.clear()
            if (id) {
                ws.connect(`/api/ws/nodes/${id}/metrics`)
            } else {
                ws.close()
            }
        },
        { immediate: true },
    )

    function pickByName(name) {
        const out = []
        for (const s of seriesMap.value.values()) {
            if (s.name === name) out.push(s)
        }
        return out
    }

    const cpuSeries = computed(() => pickByName('gameap_node_cpu_usage_percent'))
    const memoryPercentSeries = computed(() => pickByName('gameap_node_memory_usage_percent'))
    const memoryBytesSeries = computed(() => pickByName('gameap_node_memory_usage_bytes'))
    const memoryTotalSeries = computed(() => pickByName('gameap_node_memory_total_bytes'))
    const diskReadSeries = computed(() => pickByName('gameap_node_disk_read_bytes_total'))
    const diskWriteSeries = computed(() => pickByName('gameap_node_disk_write_bytes_total'))
    const diskUsageBytesSeries = computed(() => pickByName('gameap_node_disk_usage_bytes'))
    const diskUsagePercentSeries = computed(() => pickByName('gameap_node_disk_usage_percent'))
    const diskTotalBytesSeries = computed(() => pickByName('gameap_node_disk_total_bytes'))
    const networkInSeries = computed(() => pickByName('gameap_node_network_receive_bytes_total'))
    const networkOutSeries = computed(() => pickByName('gameap_node_network_transmit_bytes_total'))
    const load1Series = computed(() => pickByName('gameap_node_load1'))
    const load5Series = computed(() => pickByName('gameap_node_load5'))
    const load15Series = computed(() => pickByName('gameap_node_load15'))

    return {
        status: ws.status,
        replayDone,
        errorMessage,
        cpuSeries,
        memoryPercentSeries,
        memoryBytesSeries,
        memoryTotalSeries,
        diskReadSeries,
        diskWriteSeries,
        diskUsageBytesSeries,
        diskUsagePercentSeries,
        diskTotalBytesSeries,
        networkInSeries,
        networkOutSeries,
        load1Series,
        load5Series,
        load15Series,
        close: ws.close,
    }
}
