# Mock API System

This directory contains a complete mock API system using [Mock Service Worker (MSW)](https://mswjs.io/) to enable plugin development and testing without a live backend server.

## Files

| File | Description |
|------|-------------|
| `browser.ts` | MSW worker setup and initialization |
| `handlers.ts` | HTTP request handlers covering 40+ API endpoints |
| `servers.ts` | Mock game server data, games, and server capabilities |
| `users.ts` | Mock user profiles (admin, regular user, guest) |
| `files.ts` | Mock file system with nested directories |
| `translations-en.json` | English language strings |
| `translations-ru.json` | Russian language strings |

## Debug State

The mock system exposes a configurable debug state:

```typescript
debugState = {
    userType: 'admin' | 'user' | 'guest',  // Controls permission level
    serverId: 1 | 2 | 3,                    // Selected mock server
    locale: 'en' | 'ru',                    // UI language
    networkDelay: 100                       // Simulated latency (ms)
}
```

Update state via:
```typescript
import { updateDebugState } from './browser'
updateDebugState({ userType: 'user', networkDelay: 500 })
```

## Mock Servers

Three game servers with different states for testing:

1. **Minecraft Survival** (ID: 1) - Installed, online, running
2. **CS2 Competitive** (ID: 2) - Installed, offline, not running
3. **Rust Server** (ID: 3) - Not installed

Each server has different capabilities (RCON, console access, file manager, etc.) to test various plugin scenarios.

## API Coverage

- Auth & Profile
- Servers (list, control, console, RCON, tasks)
- Games & Mods
- Dedicated Servers / Nodes
- Users & Permissions
- Tokens & Certificates
- GDaemon Tasks
- File Manager (browse, upload, download, zip/unzip)
- Plugins (JS/CSS loading)
- Plugin Store (categories, labels, plugins, install/update/uninstall)
- Translations

## Usage

```typescript
import { startMockServiceWorker } from './browser'

await startMockServiceWorker()
// All fetch requests to /api/* are now intercepted
```

The debug panel (in main.ts) provides UI controls for switching user types, adjusting network delay, and changing locale.

## Plugin Mock API

Plugins can register custom mock handlers for their API endpoints. This is useful for:
- Adding mock endpoints for plugin-specific APIs
- Overriding default handler responses
- Testing different API scenarios

### Registering Mock Handlers

In your plugin's `onInit()` hook:

```typescript
export const myPlugin = {
    id: 'my-plugin',
    name: 'My Plugin',
    version: '1.0.0',

    onInit() {
        if (window.gameapDebug) {
            const { http, HttpResponse, delay } = window.gameapDebug.msw

            window.gameapDebug.registerMockHandlers([
                http.get('/api/plugins/my-plugin/data', async () => {
                    await delay(100)
                    return HttpResponse.json({
                        items: [{ id: 1, name: 'Item 1' }]
                    })
                }),
            ])
        }
    },
}
```

### MSW Utilities

The `window.gameapDebug.msw` object exposes:

| Utility | Description |
|---------|-------------|
| `http` | HTTP method handlers (`http.get()`, `http.post()`, etc.) |
| `HttpResponse` | Response builder for mock responses |
| `delay` | Async delay function for simulating network latency |

### Mock Data Utilities

Manipulate mock data programmatically via `window.gameapDebug.mockData`:

```javascript
// Add a custom server
const newServer = mockData.addServer({
    name: 'My Test Server',
    game_id: 'minecraft',
    process_active: true
})

// Update existing server
mockData.updateServer(1, { name: 'Updated Name' })

// Remove a server
mockData.removeServer(3)

// Get all servers
const servers = mockData.getServers()

// Add a custom user type
mockData.addUser('moderator', {
    id: 3,
    login: 'mod',
    name: 'Moderator',
    roles: ['moderator'],
    isAdmin: false,
    isAuthenticated: true
})
```

### Resetting Handlers

To restore original handlers (remove all plugin handlers):

```javascript
window.gameapDebug.resetMockHandlers()
```

### Handler Priority

Plugin handlers are prepended and checked **first** before default handlers, allowing overrides.
