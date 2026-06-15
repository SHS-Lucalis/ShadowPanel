# `plugins/` — first-party SHS plugins

> Status: **scaffold (Phase 0)**. First plugin (`arma-reforger`) lands in Phase 4.

Each subdirectory is a self-contained SHS plugin with its own backend
(Go → WASM), frontend (Vue 3 → ES module), optional game template, and
manifest.

Layout per plugin (see plan §7.2):

```
plugins/<plugin-id>/
├── plugin.yaml         # manifest: id, version, hooks, routes, permissions
├── backend/            # Go source, compiled to WASM
├── frontend/           # Vue 3 source, built to dist/plugin.mjs
├── template/           # YAML template registered with the panel (optional)
└── README.md
```

Hard rule: nothing under `plugins/` may be imported from `cmd/`,
`internal/` (except `internal/shsplugin/`), `pkg/` (except
`pkg/shspluginsdk/`), `migrations/`, `openapi/`, or `web/frontend/`.
The `make shs-lint-core` check enforces this for game-name strings.
