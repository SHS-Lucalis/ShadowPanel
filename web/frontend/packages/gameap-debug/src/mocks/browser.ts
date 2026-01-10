import { setupWorker } from 'msw/browser'
import { http, HttpResponse, delay, type RequestHandler } from 'msw'
import { handlers, debugState, setPluginContent } from './handlers'

// Store initial handlers for reset capability
const initialHandlers = [...handlers]

// Queue for handlers registered before MSW starts
let pendingHandlers: RequestHandler[] = []
let workerStarted = false

export const worker = setupWorker(...handlers)

// Export debug state and utilities for external control
export { debugState, setPluginContent }

// Export MSW utilities for plugin use
export { http, HttpResponse, delay }

// Helper to update debug state
export function updateDebugState(updates: Partial<typeof debugState>) {
    Object.assign(debugState, updates)
}

// Register custom mock handlers (prepends to take priority)
export function registerMockHandlers(customHandlers: RequestHandler[]) {
    if (!workerStarted) {
        pendingHandlers.push(...customHandlers)
    } else {
        worker.use(...customHandlers)
    }
}

// Reset handlers to initial state (removes all plugin handlers)
export function resetMockHandlers() {
    worker.resetHandlers(...initialHandlers)
}

// Start the worker
export async function startMockServiceWorker() {
    const result = await worker.start({
        onUnhandledRequest: 'bypass',
        serviceWorker: {
            url: '/mockServiceWorker.js',
        },
    })

    workerStarted = true

    // Apply any handlers that were registered before start
    if (pendingHandlers.length > 0) {
        worker.use(...pendingHandlers)
        pendingHandlers = []
    }

    return result
}
