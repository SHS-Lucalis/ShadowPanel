# `plugins/arma-reforger/` — Arma Reforger Workshop & Config Manager plugin

> Status: **scaffold (Phase 0)**. Implementation lands in Phase 4.

The first first-party SHS plugin. Scope and boundary rules are defined in
`docs/architecture/00-shs-panel-architecture-plan.md` §8.

Planned contents:

- `plugin.yaml` — manifest declaring backend hooks, frontend routes, the
  `Workshop` and `Config` server tabs (`gameFilter: ["arma-reforger"]`),
  and the `arma.workshop.read` / `arma.workshop.write` permissions.
- `backend/` — Go source compiled to WASM, implementing
  `ConfigFileHook` (renders `server.json` from the structured form),
  `VarResolverHook` (expands `{mods}`), and `AdminRoutesHook`
  (`/api/plugins/shs.arma-reforger/workshop/...`).
- `frontend/` — Vue 3 source for the Workshop and Config tabs, built to
  `dist/plugin.mjs`.
- `template/arma-reforger.yaml` — the SHS egg template registered into
  the panel's `games` + `game_mods` rows by the importer.

Boundary rules:

- This plugin must work when uninstalled — Arma Reforger servers must
  still start and stop via the regular start-command path; only the
  Workshop and structured-config UX disappears.
- No code in this directory may be referenced from core panel paths.
