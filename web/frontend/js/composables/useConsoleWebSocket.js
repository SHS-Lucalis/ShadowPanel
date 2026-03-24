import { ref } from 'vue'
import { useWebSocket } from './useWebSocket'

export function useConsoleWebSocket(serverId) {
    const output = ref('')

    const ws = useWebSocket({
        onMessage(msg) {
            if (msg.type === 'console.history') {
                output.value = msg.payload.output
            } else if (msg.type === 'console.output') {
                output.value += msg.payload.chunk
            }
        },
    })

    if (serverId) {
        ws.connect(`/api/ws/servers/${serverId}/console`)
    }

    function sendCommand(command) {
        ws.send('console.command', { command })
    }

    return {
        output,
        status: ws.status,
        sendCommand,
        close: ws.close,
    }
}
