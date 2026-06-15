# `pkg/shspluginsdk/` — public Go SDK for SHS plugin authors

> Status: **scaffold (Phase 0)**. Interfaces land in Phase 1.

Stable, semver-versioned Go interfaces that SHS plugin backends compile
against. This is the only `pkg/` directory under SHS ownership; everything
else in `pkg/` is upstream GameAP.

Hook interfaces to be defined in Phase 1 (see plan §7.3):

- `ServerLifecycleHook` — `OnInstall`, `OnStart`, `OnStop`, `OnRestart`.
- `VarResolverHook` — provide values for plugin-specific `{vars}` during
  start-command templating.
- `ConfigFileHook` — produce or transform server config files before they
  are pushed to the daemon.
- `AdminRoutesHook` — register `/api/plugins/{id}/...` REST endpoints.
- `MigrationHook` — provide goose migrations applied into a per-plugin
  prefix.
- `MetricsHook` — contribute custom server metrics to the heartbeat
  payload.

ABI versioning:

- `shs_api: "1"` in the plugin manifest binds against this SDK's `v1.x.y`
  releases.
- Breaking changes bump the major; `internal/shsplugin/` will refuse to
  load plugins built against an unsupported major.
