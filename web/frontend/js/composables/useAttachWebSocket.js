import { ref } from 'vue'
import axios from '@/config/axios'
import { useWebSocket } from './useWebSocket'

function encodeBase64(text) {
    return btoa(unescape(encodeURIComponent(text)))
}

function decodeBase64(b64) {
    return decodeURIComponent(escape(atob(b64)))
}

export function useAttachWebSocket(serverId) {
    const output = ref('')
    const attached = ref(false)
    const sessionId = ref(null)
    const closeReason = ref(null)

    const ws = useWebSocket({
        onMessage(msg) {
            if (msg.type === 'attach.started') {
                sessionId.value = msg.payload.session_id
                attached.value = true
                closeReason.value = null
            } else if (msg.type === 'attach.output') {
                output.value += decodeBase64(msg.payload.data)
            } else if (msg.type === 'attach.closed') {
                attached.value = false
                closeReason.value = msg.payload.reason
            } else if (msg.type === 'error') {
                attached.value = false
                closeReason.value = msg.payload?.message || 'unknown error'
            }
        },
        onClose() {
            attached.value = false
        },
    })

    if (serverId) {
        axios.get(`/api/servers/${serverId}/console`)
            .then((response) => {
                if (response.data?.console) {
                    output.value = response.data.console
                }
            })
            .catch(() => {})
            .finally(() => {
                ws.connect(`/api/ws/servers/${serverId}/attach`)
            })
    }

    function sendInput(text) {
        ws.send('attach.input', { data: encodeBase64(text) })
    }

    function detach() {
        ws.send('attach.detach')
    }

    return {
        output,
        attached,
        sessionId,
        closeReason,
        status: ws.status,
        sendInput,
        detach,
        close: ws.close,
    }
}
