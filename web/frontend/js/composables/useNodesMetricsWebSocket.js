import { ref, shallowRef, onMounted } from 'vue'
import { useWebSocket } from './useWebSocket'

const NAME_CPU = 'gameap_node_cpu_usage_percent'
const NAME_MEM_PERCENT = 'gameap_node_memory_usage_percent'
const NAME_MEM_BYTES = 'gameap_node_memory_usage_bytes'
const NAME_MEM_TOTAL = 'gameap_node_memory_total_bytes'
const NAME_NET_RX = 'gameap_node_network_receive_bytes_total'
const NAME_NET_TX = 'gameap_node_network_transmit_bytes_total'

function seriesKey(name, labels) {
    if (!labels) return name
    const keys = Object.keys(labels).sort()
    if (keys.length === 0) return name
    const parts = keys.map(k => `${k}=${labels[k]}`)
    return `${name}|${parts.join(',')}`
}

function emptySnapshot() {
    return {
        cpuPercent: null,
        memPercent: null,
        memBytes: null,
        memTotal: null,
        netInBps: null,
        netOutBps: null,
        lastTs: 0,
    }
}

export function useNodesMetricsWebSocket() {
    const snapshots = shallowRef(new Map())
    const errorMessage = ref('')

    const counterPrev = new Map()

    function counterRate(nodeId, name, labels, tsMs, value) {
        const key = `${nodeId}|${seriesKey(name, labels)}`
        const prev = counterPrev.get(key)
        counterPrev.set(key, { ts: tsMs, v: value })
        if (!prev) return null
        const dt = (tsMs - prev.ts) / 1000
        const dv = value - prev.v
        if (dv < 0 || dt <= 0) return null
        return dv / dt
    }

    function ingestEnvelope(env) {
        if (!env || !Array.isArray(env.series)) return
        const nodeID = env.common_labels?.node_id
        if (!nodeID) return

        const map = snapshots.value
        const snap = map.get(nodeID) || emptySnapshot()

        for (const s of env.series) {
            if (!s || !Array.isArray(s.points) || s.points.length === 0) continue
            const point = s.points[s.points.length - 1]
            const tsMs = Date.parse(point.timestamp)
            if (Number.isNaN(tsMs)) continue
            const value = parseFloat(point.value)
            if (Number.isNaN(value)) continue

            if (tsMs > snap.lastTs) snap.lastTs = tsMs

            switch (s.name) {
                case NAME_CPU:
                    snap.cpuPercent = value
                    break
                case NAME_MEM_PERCENT:
                    snap.memPercent = value
                    break
                case NAME_MEM_BYTES:
                    snap.memBytes = value
                    break
                case NAME_MEM_TOTAL:
                    snap.memTotal = value
                    break
                case NAME_NET_RX: {
                    const rate = counterRate(nodeID, s.name, s.labels, tsMs, value)
                    if (rate !== null) snap.netInBps = rate
                    break
                }
                case NAME_NET_TX: {
                    const rate = counterRate(nodeID, s.name, s.labels, tsMs, value)
                    if (rate !== null) snap.netOutBps = rate
                    break
                }
                default:
                    break
            }
        }

        if (snap.memPercent === null && snap.memBytes !== null && snap.memTotal) {
            snap.memPercent = (snap.memBytes / snap.memTotal) * 100
        }

        map.set(nodeID, snap)
        snapshots.value = new Map(map)
    }

    const ws = useWebSocket({
        onMessage(msg) {
            if (msg.type === 'metrics') {
                ingestEnvelope(msg.payload)
            } else if (msg.type === 'metrics.error') {
                errorMessage.value = msg.payload?.error || 'unknown error'
            }
        },
        onOpen() {
            errorMessage.value = ''
        },
    })

    onMounted(() => {
        ws.connect('/api/ws/nodes/metrics')
    })

    function snapshotFor(nodeId) {
        const id = String(nodeId)
        return snapshots.value.get(id) || null
    }

    return {
        status: ws.status,
        errorMessage,
        snapshots,
        snapshotFor,
        close: ws.close,
    }
}
