import type { ServerData } from '@gameap/plugin-sdk'

// Extended server type for API responses (includes nested game object)
export interface ServerListItem {
    id: number
    enabled: boolean
    installed: number // 0 = not installed, 1 = installed, 2 = installing
    blocked: boolean
    name: string
    game_id: string
    ds_id: number
    game_mod_id: number
    expires: string | null
    server_ip: string
    server_port: number
    query_port: number
    rcon_port: number
    process_active: boolean
    last_process_check: string
    game: GameInfo
    online: boolean
}

// Game info for nested game object
export interface GameInfo {
    code: string
    name: string
    engine: string
    engine_version: string
    steam_app_id_linux?: number
    steam_app_id_windows?: number
    steam_app_set_config?: string
    remote_repository_linux?: string | null
    remote_repository_windows?: string | null
    local_repository_linux?: string | null
    local_repository_windows?: string | null
    enabled?: number
}

// Full server details for /api/servers/:id
export interface ServerDetails extends ServerListItem {
    uuid: string
    uuid_short: string
    rcon: string
    dir: string
    su_user: string
    cpu_limit: number | null
    ram_limit: number | null
    net_limit: number | null
    start_command: string
    stop_command: string | null
    force_stop_command: string | null
    restart_command: string | null
    aliases: Record<string, string | number>
    vars: Record<string, string> | null
    created_at: string
    updated_at: string
}

// Mock games with full info
const minecraftGame: GameInfo = {
    code: 'minecraft',
    name: 'Minecraft',
    engine: 'Minecraft',
    engine_version: '1',
    steam_app_id_linux: 0,
    steam_app_id_windows: 0,
    steam_app_set_config: '',
    remote_repository_linux: 'http://packages.gameap.com/mcrun/mcrun-v1.2.0-linux-amd64.tar.gz',
    remote_repository_windows: 'http://packages.gameap.com/mcrun/mcrun-v1.2.0-windows-amd64.zip',
    local_repository_linux: null,
    local_repository_windows: null,
    enabled: 1,
}

const cs2Game: GameInfo = {
    code: 'cs2',
    name: 'Counter-Strike 2',
    engine: 'Source2',
    engine_version: '1',
    steam_app_id_linux: 730,
    steam_app_id_windows: 730,
    steam_app_set_config: '',
    remote_repository_linux: null,
    remote_repository_windows: null,
    local_repository_linux: null,
    local_repository_windows: null,
    enabled: 1,
}

const rustGame: GameInfo = {
    code: 'rust',
    name: 'Rust',
    engine: 'Unity',
    engine_version: '1',
    steam_app_id_linux: 258550,
    steam_app_id_windows: 258550,
    steam_app_set_config: '',
    remote_repository_linux: null,
    remote_repository_windows: null,
    local_repository_linux: null,
    local_repository_windows: null,
    enabled: 1,
}

// Mock servers for list API
export const mockServersList: ServerListItem[] = [
    {
        id: 1,
        enabled: true,
        installed: 1,
        blocked: false,
        name: 'Minecraft Survival',
        game_id: 'minecraft',
        ds_id: 1,
        game_mod_id: 1,
        expires: null,
        server_ip: '192.168.1.100',
        server_port: 25565,
        query_port: 25565,
        rcon_port: 25575,
        process_active: true,
        last_process_check: new Date().toISOString(),
        game: minecraftGame,
        online: true,
    },
    {
        id: 2,
        enabled: true,
        installed: 1,
        blocked: false,
        name: 'CS2 Competitive',
        game_id: 'cs2',
        ds_id: 1,
        game_mod_id: 2,
        expires: null,
        server_ip: '192.168.1.101',
        server_port: 27015,
        query_port: 27015,
        rcon_port: 27015,
        process_active: false,
        last_process_check: new Date().toISOString(),
        game: cs2Game,
        online: false,
    },
    {
        id: 3,
        enabled: true,
        installed: 0, // Not installed
        blocked: false,
        name: 'Rust Server',
        game_id: 'rust',
        ds_id: 1,
        game_mod_id: 3,
        expires: null,
        server_ip: '192.168.1.102',
        server_port: 28015,
        query_port: 28016,
        rcon_port: 28016,
        process_active: false,
        last_process_check: new Date().toISOString(),
        game: rustGame,
        online: false,
    },
]

// Full server details for /api/servers/:id
export const mockServersDetails: Record<number, ServerDetails> = {
    1: {
        id: 1,
        uuid: '019a1234-abcd-7000-8000-000000000001',
        uuid_short: '019a1234',
        enabled: true,
        installed: 1,
        blocked: false,
        name: 'Minecraft Survival',
        game_id: 'minecraft',
        ds_id: 1,
        game_mod_id: 1,
        expires: null,
        server_ip: '192.168.1.100',
        server_port: 25565,
        query_port: 25565,
        rcon_port: 25575,
        game: minecraftGame,
        last_process_check: new Date().toISOString(),
        online: true,
        rcon: 'rcon_password_123',
        dir: 'servers/019a1234-abcd-7000-8000-000000000001',
        su_user: 'gameap',
        cpu_limit: null,
        ram_limit: null,
        net_limit: null,
        start_command: './mcrun run --version={version} --mod={mod} --ip={ip} --port={port} --query-port={query_port} --rcon-port={rcon_port} --rcon-password={rcon_password}',
        stop_command: null,
        force_stop_command: null,
        restart_command: null,
        process_active: true,
        aliases: {
            ip: '192.168.1.100',
            port: 25565,
            query_port: 25565,
            rcon_password: 'rcon_password_123',
            rcon_port: 25575,
            uuid: '019a1234-abcd-7000-8000-000000000001',
            uuid_short: '019a1234',
        },
        vars: {
            version: '1.20.4',
            mod: 'vanilla',
        },
        created_at: '2024-01-01T10:00:00+00:00',
        updated_at: '2024-12-17T12:00:00+00:00',
    },
    2: {
        id: 2,
        uuid: '019a5678-efgh-7000-8000-000000000002',
        uuid_short: '019a5678',
        enabled: true,
        installed: 1,
        blocked: false,
        name: 'CS2 Competitive',
        game_id: 'cs2',
        ds_id: 1,
        game_mod_id: 2,
        expires: null,
        server_ip: '192.168.1.101',
        server_port: 27015,
        query_port: 27015,
        rcon_port: 27015,
        game: cs2Game,
        last_process_check: new Date().toISOString(),
        online: false,
        rcon: 'cs2_rcon_pass',
        dir: 'servers/019a5678-efgh-7000-8000-000000000002',
        su_user: 'gameap',
        cpu_limit: null,
        ram_limit: null,
        net_limit: null,
        start_command: './cs2 -dedicated +map de_dust2 +maxplayers 20 -port {port}',
        stop_command: null,
        force_stop_command: null,
        restart_command: null,
        process_active: false,
        aliases: {
            ip: '192.168.1.101',
            port: 27015,
            query_port: 27015,
            rcon_password: 'cs2_rcon_pass',
            rcon_port: 27015,
            uuid: '019a5678-efgh-7000-8000-000000000002',
            uuid_short: '019a5678',
        },
        vars: {
            maxplayers: '20',
            map: 'de_dust2',
        },
        created_at: '2024-02-15T14:00:00+00:00',
        updated_at: '2024-12-17T12:00:00+00:00',
    },
    3: {
        id: 3,
        uuid: '019a9012-ijkl-7000-8000-000000000003',
        uuid_short: '019a9012',
        enabled: true,
        installed: 0,
        blocked: false,
        name: 'Rust Server',
        game_id: 'rust',
        ds_id: 1,
        game_mod_id: 3,
        expires: null,
        server_ip: '192.168.1.102',
        server_port: 28015,
        query_port: 28016,
        rcon_port: 28016,
        game: rustGame,
        last_process_check: new Date().toISOString(),
        online: false,
        rcon: 'rust_rcon_pass',
        dir: 'servers/019a9012-ijkl-7000-8000-000000000003',
        su_user: 'gameap',
        cpu_limit: null,
        ram_limit: null,
        net_limit: null,
        start_command: './RustDedicated -batchmode +server.port {port} +rcon.port {rcon_port} +rcon.password {rcon_password}',
        stop_command: null,
        force_stop_command: null,
        restart_command: null,
        process_active: false,
        aliases: {
            ip: '192.168.1.102',
            port: 28015,
            query_port: 28016,
            rcon_password: 'rust_rcon_pass',
            rcon_port: 28016,
            uuid: '019a9012-ijkl-7000-8000-000000000003',
            uuid_short: '019a9012',
        },
        vars: null,
        created_at: '2024-03-20T09:00:00+00:00',
        updated_at: '2024-12-17T12:00:00+00:00',
    },
}

// Server data for plugin SDK (used by useServer hook)
export const minecraftServer: ServerData = {
    id: 1,
    uuid: '019a1234-abcd-7000-8000-000000000001',
    name: 'Minecraft Survival',
    game_id: 'minecraft',
    game_mod_id: 1,
    ip: '192.168.1.100',
    port: 25565,
    query_port: 25565,
    rcon_port: 25575,
    enabled: true,
    installed: true,
    blocked: false,
    start_command: './mcrun run --version={version} --mod={mod} --ip={ip} --port={port}',
    dir: 'servers/019a1234-abcd-7000-8000-000000000001',
    process_active: true,
    last_process_check: new Date().toISOString(),
}

export const csServer: ServerData = {
    id: 2,
    uuid: '019a5678-efgh-7000-8000-000000000002',
    name: 'CS2 Competitive',
    game_id: 'cs2',
    game_mod_id: 2,
    ip: '192.168.1.101',
    port: 27015,
    query_port: 27015,
    rcon_port: 27015,
    enabled: true,
    installed: true,
    blocked: false,
    start_command: './cs2 -dedicated +map de_dust2 +maxplayers 20 -port {port}',
    dir: 'servers/019a5678-efgh-7000-8000-000000000002',
    process_active: false,
    last_process_check: new Date().toISOString(),
}

export const serverMocks: Record<string, ServerData> = {
    minecraft: minecraftServer,
    cs: csServer,
}

// Server abilities as object with boolean values (matching real API format)
export const serverAbilities: Record<number, Record<string, boolean>> = {
    1: { // Minecraft
        'game-server-common': true,
        'game-server-console-send': true,
        'game-server-console-view': true,
        'game-server-files': true,
        'game-server-pause': false,
        'game-server-rcon-console': false,
        'game-server-rcon-players': false,
        'game-server-restart': true,
        'game-server-settings': true,
        'game-server-start': true,
        'game-server-stop': true,
        'game-server-tasks': true,
        'game-server-update': true,
    },
    2: { // CS2
        'game-server-common': true,
        'game-server-console-send': true,
        'game-server-console-view': true,
        'game-server-files': true,
        'game-server-pause': false,
        'game-server-rcon-console': true,
        'game-server-rcon-players': true,
        'game-server-restart': true,
        'game-server-settings': true,
        'game-server-start': true,
        'game-server-stop': true,
        'game-server-tasks': true,
        'game-server-update': true,
    },
    3: { // Rust (not installed)
        'game-server-common': true,
        'game-server-console-send': false,
        'game-server-console-view': false,
        'game-server-files': false,
        'game-server-pause': false,
        'game-server-rcon-console': false,
        'game-server-rcon-players': false,
        'game-server-restart': false,
        'game-server-settings': true,
        'game-server-start': false,
        'game-server-stop': false,
        'game-server-tasks': false,
        'game-server-update': true,
    },
}
