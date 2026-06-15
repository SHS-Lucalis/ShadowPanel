# SHS Panel — Architecture Plan v0.1

Status: **Draft for review**
Base project: [gameap/gameap](https://github.com/gameap/gameap) (MIT, Go 1.26, Vue 3)
Inspiration only (not copied): [pelican-dev/panel](https://github.com/pelican-dev/panel), [pterodactyl/panel](https://github.com/pterodactyl/panel)

---

## 1. TL;DR — What SHS Panel Is

SHS Panel is a **fork of modern (Go) GameAP** that:

1. Keeps GameAP's Go backend, gRPC daemon, native Windows/Linux process management, and database schema. These are already production-grade and Docker-optional.
2. Replaces / restyles the GameAP Vue 3 frontend into a Pelican-style modern dark UI (server cards, live console, file manager, variable editor).
3. Formalises GameAP's existing-but-undocumented plugin system (frontend WASM SDK + backend hook points) into a **stable SHS Plugin API** with documented backend hooks and frontend route injection.
4. Ships **Arma Reforger Workshop / Config Manager** as the first first-party plugin — never hardcoded into core.

**Key audit finding that drives this plan:** the user's request was written assuming legacy PHP/Laravel GameAP. GameAP has been rewritten in Go (commit history < 12 months, 28 releases, MIT). Almost every "core requirement" in the brief — Windows/Linux native, no-Docker, websocket console, template/vars system, plugin loader — already exists upstream. The differentiation work for SHS is **UI/UX, plugin DX, and game-specific plugins**, not rebuilding daemon or backend.

---

## 2. Decision Record

| # | Decision | Reason |
|---|---|---|
| D1 | Fork `gameap/gameap` rather than starting from Pelican or Pterodactyl | GameAP is the only one of the three with native Windows + Linux support and no Docker dependency. Pelican and Pterodactyl are Docker-mandatory. |
| D2 | Do **not** merge Pelican/Pterodactyl code | License of Pterodactyl is restrictive; merging Pelican PHP/Laravel into GameAP Go is incompatible. We mine concepts (eggs, dark UI) only. |
| D3 | Keep GameAP's Go backend, gRPC daemon, Vue 3 + Vite frontend stack | Already modern. Replacing them is a multi-month risk for zero functional gain. |
| D4 | Replace Naive UI component library with shadcn-vue + Tailwind dark theme | Required for the "Pelican-inspired modern dark UI" mandate. |
| D5 | Treat existing GameAP `games` + `game_mods` rows as the template store | Avoids inventing a parallel "egg" system; just add an importer that converts a Pelican egg JSON into a `games` + `game_mods` row. |
| D6 | Plugin layer = formalise + document existing `knqyf263/go-plugin` (WASM) hooks + Vue plugin SDK | Don't invent a new plugin loader. Add explicit backend hook interfaces and version the frontend SDK. |
| D7 | Arma Reforger support lives in a plugin (`shs-plugin-arma-reforger`) and a game template, never in core | Hard rule from the brief. |
| D8 | Rebrand to SHS Panel; keep `gameap.com` plugin store URL configurable so we can point at our own | Allows dual ecosystem during transition. |

---

## 3. Stack at a Glance (Inherited from GameAP)

| Layer | Tech | Notes |
|---|---|---|
| Panel backend | Go 1.26 single binary | Layout: `cmd/`, `internal/api`, `internal/domain`, `internal/repositories`, `internal/ws`, `pkg/` |
| Database | PostgreSQL / MySQL / SQLite (driver via `DATABASE_DRIVER`) | SQLite is the zero-setup default; no Redis required for single-node |
| Auth | PASETO (default) or JWT, RBAC, personal access tokens | `AUTH_SERVICE=paseto` |
| Daemon | Separate Go binary (`gameap/daemon`), gRPC over TLS / mTLS | Long-lived bidi stream; `coder/websocket` on the panel side bridges to browsers |
| Process manager (Linux) | systemd user / system units, optional cgroup limits | `internal/processmanager/systemd.go` |
| Process manager (Windows) | `shawl.exe` (preferred) or WinSW via `sc.exe` | `internal/processmanager/shawl_windows.go`, `winsw_windows.go` |
| Install pipeline | SteamCMD wrapper, optional shell scripts, archive download+extract | Templated `start_command`, `stop_command`, `vars` substitution |
| Frontend | Vue 3 + Vite + Pinia + Tailwind, currently Naive UI | `web/frontend/`, plugin SDK at `web/plugin-sdk/` |
| WebSocket | `coder/websocket` on panel; gRPC `attach` stream from daemon | Interactive (stdin + stdout), per-server session pool |
| Plugin loader | `knqyf263/go-plugin` (WASM via `tetratelabs/wazero`) for backend; Vite-built Vue chunks for frontend | Backend hooks are runtime-loaded WASM; frontend plugins are dynamically imported ES modules |
| File storage | local FS or S3 (via `FILES_DRIVER`) | `os.Root` confinement on the daemon side prevents path traversal |
| TLS | Built-in ACME / Let's Encrypt via `go-acme/lego` | No external certbot |

License: **MIT** — derivative work, rebrand, and commercial use are all permitted with attribution.

---

## 4. Architecture Diagram

```mermaid
flowchart LR
    subgraph Browser
      UI[SHS Panel SPA<br/>Vue 3 + shadcn-vue dark theme]
      PluginsFE[Frontend Plugins<br/>e.g. Arma Reforger UI]
    end

    subgraph PanelHost[Panel Host (Linux/Windows/macOS)]
      API[REST API<br/>Go]
      WS[WebSocket Bridge<br/>coder/websocket]
      Hooks[Backend Plugin Host<br/>wazero + go-plugin]
      DB[(SQLite / Postgres / MySQL)]
      FS[(Local FS or S3)]
    end

    subgraph NodeA[Game Server Node A — Linux]
      DaemonA[GameAP Daemon<br/>Go]
      Sysd[(systemd user units)]
      ProcA[Game Process<br/>e.g. Arma Reforger Server]
    end

    subgraph NodeB[Game Server Node B — Windows]
      DaemonB[GameAP Daemon<br/>Go]
      Shawl[(shawl.exe / WinSW)]
      ProcB[Game Process]
    end

    UI <-- HTTPS REST + WSS --> API
    UI <-- WSS console stream --> WS
    PluginsFE -. injects routes/tabs .-> UI
    API <-->|RBAC + persistence| DB
    API <--> Hooks
    Hooks -. WASM ABI .-> SHSPlugins[SHS Backend Plugins<br/>e.g. Arma Reforger Workshop]
    API <-- gRPC mTLS bidi stream --> DaemonA
    API <-- gRPC mTLS bidi stream --> DaemonB
    WS <-- attach stream --> DaemonA
    WS <-- attach stream --> DaemonB
    DaemonA --> Sysd --> ProcA
    DaemonB --> Shawl --> ProcB
    API --- FS
```

---

## 5. Audit Map — What Already Exists Upstream

For each "core requirement" the user listed, this table shows where it lives in GameAP today, so SHS work isn't duplicated.

| User requirement | Already in GameAP? | Where | SHS action |
|---|---|---|---|
| 1. Windows + Linux node support | Yes | `internal/processmanager/{systemd.go,shawl_windows.go,winsw_windows.go}` | Reuse. Add CI matrix to keep both paths green. |
| 2. No Docker requirement | Yes (Docker is one deploy option, not mandatory) | `Dockerfile` is opt-in; binary install via `gameapctl panel install` is documented and primary | Reuse. Document the native install path more prominently. |
| 3. Native process management | Yes | Same as #1 | Reuse. |
| 4. WebSocket live console | Yes | Panel: `internal/ws/` + `internal/app/grpc/attach_handler.go`. Daemon: gRPC `Attach` stream. | Reuse, restyle the frontend console widget. |
| 5. Template system (install / start / update / config / vars) | Yes (DB-driven, not YAML-first) | Tables `games`, `game_mods`. Vars are JSON in `game_mods.vars`. Start cmd in `game_mods.default_start_cmd`. Install via SteamCMD or remote repo. | Add a YAML egg importer that maps Pelican-egg-like YAML → these rows. Don't replace the schema. |
| 6. Plugin system (backend hooks + frontend routes) | **Partial** | Frontend: `web/plugin-sdk/` (typed Vue SDK). Backend: `pkg/plugin/dispatcher.go` + `knqyf263/go-plugin` are present but hook surface is undocumented and incomplete. | This is SHS's main engineering investment. See §7. |
| 7. First plugin: Arma Reforger Workshop/config manager | No | — | Build as `shs-plugin-arma-reforger`. See §8. |
| 8. Preserve GameAP server management | — | Entire `internal/services/` tree | Reuse; no rewrites. |
| 9. Pelican as inspiration only | — | — | UI/UX cues (cards, console, variable editor look-and-feel). Egg YAML structure inspires the importer in #5. No code is lifted. |
| 10. No hardcoded Arma Reforger in core | — | — | Lint rule: PRs that touch `internal/` referencing `arma`, `reforger`, `workshop`, `bohemia` are blocked. All such logic lives under `plugins/arma-reforger/`. |

---

## 6. Repository Layout (Proposed)

We fork GameAP and add SHS-owned directories alongside the upstream tree. Upstream paths are left intact so future merges from `gameap/gameap` stay tractable.

```
shspanel/panel
├── cmd/                       # upstream (Go entrypoints)
├── internal/                  # upstream Go backend — modify with care
│   ├── api/                   # REST handlers
│   ├── domain/                # models (Server, Node, Game, GameMod, …)
│   ├── repositories/          # data access
│   ├── services/              # business logic (install, start, config push, …)
│   ├── ws/                    # WebSocket bridge
│   └── shsplugin/             # NEW — SHS plugin host: hook registry, ABI versioning
├── pkg/
│   ├── plugin/                # upstream WASM dispatcher (kept)
│   └── shspluginsdk/          # NEW — Go interface definitions exported to plugin authors
├── web/
│   ├── frontend/              # upstream Vue 3 SPA — restyled in place under feature flag
│   ├── plugin-sdk/            # upstream frontend SDK — extended, semver-stable
│   └── shs-theme/             # NEW — Tailwind config, shadcn-vue components, dark tokens
├── plugins/                   # NEW — first-party SHS plugins (each is its own Go module + Vue package)
│   └── arma-reforger/
│       ├── backend/           # Go → compiled to WASM
│       ├── frontend/          # Vue 3 — built to ES module bundle
│       ├── template/          # YAML egg + install/start/config rules
│       └── plugin.yaml        # manifest: id, version, hooks, routes, permissions
├── templates/                 # NEW — first-party game template YAMLs (importable into games/game_mods)
│   ├── arma-reforger.yaml
│   └── _schema.json
├── docs/
│   ├── architecture/          # this folder
│   ├── plugin-authoring/      # plugin developer guide
│   └── upgrade-from-gameap/   # for users switching from upstream
└── .shs/                      # NEW — branding assets, plugin store URL, tooling config
```

---

## 7. SHS Plugin Architecture

This is the section that does new work; everything else is reuse.

### 7.1 Goals

- Plugins can ship **backend logic, frontend UI, a game template, and a manifest** as a single unit.
- Plugins are loaded at runtime without restarting the panel where possible.
- A plugin can never bypass RBAC, escape the file confinement (`os.Root`), or modify other plugins' state.
- Frontend hot-reload: dropping a plugin bundle into `data/plugins/` and clicking "Reload" updates routes without a full restart.
- Backend ABI is versioned. Plugins declare `shs_api: "1"` in their manifest; the panel refuses incompatible plugins.

### 7.2 Plugin manifest (`plugin.yaml`)

```yaml
id: shs.arma-reforger                # reverse-DNS, globally unique
name: Arma Reforger Workshop Manager
version: 0.1.0
shs_api: "1"                         # SHS plugin ABI major version
authors: [SHS Studio]
license: MIT

# What backend hooks this plugin implements (see §7.3)
backend:
  module: backend/plugin.wasm
  hooks:
    - server.lifecycle            # start/stop/install events
    - server.vars.resolve         # custom variable resolver
    - server.config.push          # config file generation
    - admin.routes                # adds /api/plugins/arma-reforger/* endpoints

# What frontend the plugin contributes
frontend:
  bundle: frontend/dist/plugin.mjs
  routes:
    - path: /servers/:serverId/arma/workshop
      component: WorkshopManager
      requires: server.read
    - path: /admin/arma-reforger
      component: AdminSettings
      requires: admin
  serverTabs:
    - id: workshop
      label: Workshop
      gameFilter: ["arma-reforger"]   # tab only shows for matching game code
      component: WorkshopTab

# Game templates this plugin registers (optional)
templates:
  - templates/arma-reforger.yaml

# Permissions the plugin needs (RBAC abilities it defines)
permissions:
  - id: arma.workshop.read
    description: View installed workshop mods
  - id: arma.workshop.write
    description: Install/remove workshop mods
```

### 7.3 Backend hook surface (v1)

Defined as Go interfaces in `pkg/shspluginsdk/`. Each is exposed across the WASM boundary via `knqyf263/go-plugin` proto definitions.

| Hook | When it fires | Plugin can | Implemented in |
|---|---|---|---|
| `ServerLifecycleHook.OnInstall(server, game)` | Before / after a server install task is dispatched | Mutate install rules, abort with reason | `internal/services/installation/` |
| `ServerLifecycleHook.OnStart/OnStop/OnRestart` | Around a daemon command dispatch | Wrap commands, inject env, abort with reason | `internal/services/gameserver/` |
| `VarResolverHook.Resolve(server, varName)` | During start-command templating | Provide values for plugin-specific `{vars}` | `internal/api/daemonapi/servers/getserverid/response.go` |
| `ConfigFileHook.Generate(server, configFile)` | When a config file is pushed to the daemon | Produce or transform file content | `internal/services/serverconfigpush/pusher.go` |
| `AdminRoutesHook.Register(router)` | At panel startup | Add `/api/plugins/{id}/...` REST endpoints | new file `internal/shsplugin/router.go` |
| `MigrationHook.Migrations()` | At panel startup | Provide goose migrations applied into a per-plugin schema/prefix | new |
| `MetricsHook.Collect(server)` | On heartbeat | Add custom server metrics surfaced in UI | `internal/ws/bridge.go` |

Hooks are **opt-in**: a plugin only declares the ones it needs in `plugin.yaml`. The panel calls only declared hooks, so stale plugins don't pay overhead.

### 7.4 Frontend SDK extensions (additive on top of `web/plugin-sdk/`)

```ts
// web/plugin-sdk/src/index.ts (extended)
export interface PluginDefinition {
  id: string;
  routes?: PluginRoute[];
  serverTabs?: PluginServerTab[];        // NEW
  serverCardSlots?: PluginCardSlot[];    // NEW — render badges/buttons on server cards
  adminMenuItems?: PluginMenuItem[];     // NEW
  init?(ctx: PluginContext): void | Promise<void>;
}

export interface PluginContext {
  api: PluginApiClient;        // typed REST client to /api/plugins/{id}/*
  ws: PluginWsClient;          // multiplexed channel on the panel's existing WSS
  user: () => UserSnapshot;
  server: () => ServerSnapshot | null;
  toast: ToastApi;
  i18n: I18nApi;
}
```

`gameFilter` on tabs and card slots lets the Arma Reforger plugin appear only on Arma Reforger servers.

### 7.5 Plugin store

- `PLUGIN_STORE_URL` already exists in upstream config (`https://plugins.gameap.dev/api`).
- SHS ships its own store URL by default but keeps it overridable, so SHS users keep access to upstream plugins.

---

## 8. First-party plugin: `shs-plugin-arma-reforger`

The user named this as plugin #1. Concrete plan:

### 8.1 What it provides

- **Game template** (`templates/arma-reforger.yaml`): SteamCMD app id, default start command for Linux + Windows, stop command, default `server.json` config skeleton, vars (`port`, `maxPlayers`, `scenarioId`, `password`, `adminPassword`, `mods`, `bindAddress`, …).
- **Frontend tab "Workshop"** on Arma Reforger servers: search Bohemia Workshop, install / pin / remove mods, drag-reorder load order, see disk usage.
- **Frontend tab "Config"**: structured editor for `server.json` (game properties, scenario, mission rotation) with validation; falls back to raw JSON view.
- **Backend**: implements `ConfigFileHook` to render `server.json` from the structured form, `VarResolverHook` for `{mods}` expansion, `AdminRoutesHook` for `/api/plugins/shs.arma-reforger/workshop/search` etc.

### 8.2 Boundary rules

- Core panel must not import anything from `plugins/arma-reforger/`.
- Core panel must not contain plugin-specific identifiers — concretely, the strings `arma-reforger` (the game code) and `shs.arma-reforger` (the plugin id) outside of `templates/` and `plugins/`. Enforced by `make shs-lint-core` (`.shs/tools/shslint/`). The check intentionally does **not** ban generic words like `arma` or `workshop`, because those legitimately appear in upstream icon fonts, the GameAP global-games-API fixtures (which list Arma 3 etc.), and other generic infrastructure. New plugins add their own ids to `.shs/lint/forbidden-strings.txt`.
- The plugin must work when uninstalled — i.e. an Arma Reforger server still starts and stops via the regular start-command path; only the Workshop / structured-config UX disappears.

---

## 9. UI / UX direction

Pelican-inspired, not Pelican-cloned. Keep Vue 3 + Vite + Tailwind from upstream; swap the component library.

| Area | Current GameAP | SHS direction |
|---|---|---|
| Component library | Naive UI | shadcn-vue (Radix-Vue + Tailwind) |
| Theme | Light + dark, light is default | Dark by default, light available |
| Server list | Table | Server cards with status pill, CPU / RAM bar, quick start/stop |
| Console | Functional, plain | Monospace console card with ANSI color, autoscroll lock, command history (↑/↓), token-coloured stdin prompt |
| File manager | Existing tree + editor | Keep tree, add Monaco editor with syntax detection from extension |
| Variable editor | JSON-ish form | Generated form from `game_mods.vars` schema with regex validation, hover help, defaults |
| Navigation | Top bar | Left rail with server switcher + plugin-injected menu items |

UI rework is staged behind a `SHS_THEME=true` env flag during transition so we can ship incrementally without breaking upstream parity.

---

## 10. Phased execution plan (no time estimates)

**Phase 0 — Fork and brand**
- Fork `gameap/gameap` to the SHS org.
- Add `docs/architecture/` (this file).
- CI: lint + test parity with upstream; add `make shs-lint` for the "no game-name in core" rule.

**Phase 1 — Plugin host hardening**
- Add `internal/shsplugin/` host: manifest loader, hook registry, ABI version check.
- Define `pkg/shspluginsdk/` Go interfaces for hooks listed in §7.3.
- Wire hook call sites in `internal/services/installation/`, `gameserver/`, `serverconfigpush/`, and `internal/api/daemonapi/servers/getserverid/response.go` — each call site is a thin "if any plugin registered this hook, dispatch" guard.
- Frontend: extend `web/plugin-sdk/` with `serverTabs`, `serverCardSlots`, `adminMenuItems`. Cut a `1.0.0` SDK release.

**Phase 2 — Egg/template importer**
- `shs templates import path/to/egg.yaml` CLI: parses an SHS egg YAML (Pelican-shaped, but extended with `script.linux` / `script.windows` and a `process_manager` hint) and upserts `games` + `game_mods` rows.
- Ship `templates/_schema.json` and reference templates for at least two games beyond Arma Reforger to prove the abstraction.

**Phase 3 — UI restyle behind flag**
- Add `web/shs-theme/` with Tailwind tokens + shadcn-vue components.
- Restyle, in order: login → server list (cards) → server detail (header, tabs) → console → file manager → variable editor → admin pages.
- Flip default to `SHS_THEME=true` once parity is reached.

**Phase 4 — Arma Reforger plugin**
- `templates/arma-reforger.yaml` lands first; servers can already be created and run before the plugin exists.
- Backend WASM plugin with `ConfigFileHook` + `VarResolverHook` + `AdminRoutesHook`.
- Frontend tab bundle for Workshop + Config.

**Phase 5 — Plugin store**
- Decide own store URL vs proxying upstream.
- Signing + checksum verification for installed bundles.

---

## 11. Risks and open questions

| Risk | Mitigation |
|---|---|
| Upstream GameAP merges drift away from SHS | Keep all SHS code under `internal/shsplugin/`, `pkg/shspluginsdk/`, `web/shs-theme/`, `plugins/`, `templates/`. Avoid editing upstream files except at hook call sites, and keep those edits minimal (one guarded function call per site). |
| Backend WASM ABI churn | Version the SDK (`shs_api` in manifest). Refuse to load mismatched plugins. Provide a compatibility shim layer when bumping. |
| Frontend plugin module loading vs. CSP | Ship plugins as ES modules served from same-origin `/plugins/{id}/...` to keep CSP strict (`script-src 'self'`). |
| GameAP daemon protocol (gRPC) is undocumented externally | Pin a known-good daemon version per panel release. Vendor `pkg/proto/` so plugin authors and SHS contributors have one source of truth. |
| Naive UI → shadcn-vue migration breaks existing pages | `SHS_THEME` flag gates the new theme; keep both component libraries side-by-side until cutover. |
| Arma Reforger Workshop API is unofficial / may rate-limit | Plugin caches results, supports mod IDs as fallback, and never hard-fails core server start if the workshop API is down. |

**Open questions for you:**

1. SHS Studio governance: do plugins require code signing for the SHS plugin store, or is name-spaced trust (verified publishers) enough for v1?
2. Multi-tenant: is SHS Panel meant for self-hosted single-org or do we need GameAP-style per-user server quotas + node assignment from day one?
3. Branding: keep `gameap` config keys (`GLOBAL_API_URL`, `PLUGIN_STORE_URL`) for upgrade compatibility, or rename to `SHS_*` immediately and ship a one-time migrator?
4. Daemon: do we fork `gameap/daemon` too, or pin upstream and ship our daemon-side hooks (if any) as a small companion binary `shs-daemon-agent`?

---

## 12. What we are explicitly **not** building

- A new daemon — upstream is fine.
- A new auth system — PASETO + RBAC stays.
- A new database ORM or migration tool — `goose` + `squirrel` stay.
- A new realtime layer — `coder/websocket` + gRPC `Attach` stays.
- A new build system — `make` + Vite stay.
- Docker as a hard dependency — opt-in only.
- Any Arma Reforger code in `internal/` or `cmd/`.

---

_End of plan v0.1. Reviewer: please answer the four open questions in §11 before we open Phase 1 tickets._
