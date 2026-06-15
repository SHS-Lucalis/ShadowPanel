# `templates/` — SHS game templates (egg-style)

> Status: **scaffold (Phase 0)**. Importer + first templates land in Phase 2.

Pelican-inspired YAML templates that the SHS importer turns into rows in
the panel's `games` and `game_mods` tables. See architecture plan §5
("Template system") and §10 Phase 2.

A template defines:

- Game identity (`code`, `name`, `engine`, `steam_app_id_linux/windows`).
- Default install rules (SteamCMD or remote archive).
- Default start / stop / restart commands per OS.
- Variable schema (name, default, regex, description) used by the
  variable editor in the UI.
- Optional config-file scaffolds.

Schema:

- `_schema.json` (added in Phase 2) is the JSON Schema used by
  `shs templates validate` and CI.

Boundary rules:

- Templates are **data**, not code. They may name specific games. Core
  panel code (`internal/`, `pkg/` except `pkg/shspluginsdk/`) must not.
