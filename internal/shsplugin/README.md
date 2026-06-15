# `internal/shsplugin/` — SHS plugin host (panel-side)

> Status: **scaffold (Phase 0)**. Implementation lands in Phase 1.

This package is the panel-side host for the SHS plugin system described in
`docs/architecture/00-shs-panel-architecture-plan.md` §7.

Responsibilities (to be implemented):

- Discover plugin manifests (`plugin.yaml`) under the configured plugin
  directory and the plugin store.
- Validate manifest `shs_api` major version against the panel's supported
  ABI (refuse incompatible plugins).
- Maintain a runtime hook registry. Each hook listed in §7.3 of the plan has
  a typed Go interface in `pkg/shspluginsdk/`; this package owns dispatching
  to all plugins that implement a given hook.
- Bridge between the upstream `pkg/plugin/` WASM dispatcher
  (`knqyf263/go-plugin` + `tetratelabs/wazero`) and SHS's hook surface so
  plugin authors do not write WASM glue by hand.
- Mount plugin-declared admin routes under `/api/plugins/{id}/...` and
  enforce per-plugin RBAC permissions defined in the manifest.

Why this lives here, not in `pkg/plugin/`:

- `pkg/plugin/` is upstream GameAP and may change at any time.
- All SHS-owned code is kept in SHS-namespaced directories so upstream
  merges stay clean (architecture plan §11).
