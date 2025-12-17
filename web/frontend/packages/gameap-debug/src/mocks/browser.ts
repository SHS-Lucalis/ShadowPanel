import { setupWorker } from 'msw/browser'
import { handlers, debugState, setPluginContent } from './handlers'

export const worker = setupWorker(...handlers)

// Export debug state and utilities for external control
export { debugState, setPluginContent }

// Helper to update debug state
export function updateDebugState(updates: Partial<typeof debugState>) {
    Object.assign(debugState, updates)
}

// Start the worker
export async function startMockServiceWorker() {
    return worker.start({
        onUnhandledRequest: 'bypass', // Don't warn about unhandled requests
        serviceWorker: {
            url: '/mockServiceWorker.js',
        },
    })
}
