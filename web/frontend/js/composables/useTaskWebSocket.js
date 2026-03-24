import { ref, computed, watch, unref } from 'vue'
import { useWebSocket } from './useWebSocket'

const TERMINAL_STATUSES = ['success', 'error', 'canceled']

export function useTaskWebSocket(taskId, options = {}) {
    const {
        onStatusChange,
        onOutput,
        onComplete,
    } = options

    const taskStatus = ref(null)
    const taskOutput = ref('')

    const ws = useWebSocket({
        onMessage(msg) {
            if (msg.type === 'task.status') {
                taskStatus.value = msg.payload.status
                onStatusChange?.(msg.payload.status, msg.payload)
            } else if (msg.type === 'task.output') {
                taskOutput.value += msg.payload.chunk
                onOutput?.(msg.payload.chunk, msg.payload.is_final)
            } else if (msg.type === 'task.complete') {
                taskStatus.value = msg.payload.status
                onComplete?.(msg.payload.status)
                ws.close()
            }
        },
    })

    const isComplete = computed(() => TERMINAL_STATUSES.includes(taskStatus.value))

    watch(
        () => unref(taskId),
        (newId) => {
            if (newId) {
                ws.connect(`/api/ws/tasks/${newId}`)
            } else {
                ws.close()
                taskOutput.value = ''
                taskStatus.value = null
            }
        },
        { immediate: true },
    )

    return {
        taskStatus,
        taskOutput,
        isComplete,
        status: ws.status,
        close: ws.close,
    }
}
