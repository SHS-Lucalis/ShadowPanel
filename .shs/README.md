# `.shs/` — SHS Studio tooling and branding

This directory holds SHS-specific configuration that is intentionally kept
**outside** the upstream GameAP tree so future merges from `gameap/gameap`
stay tractable.

Layout:

- `lint/` — config for SHS-only lint rules (e.g. forbidden strings in core).
- `branding/` — logos, colour tokens, and any other visual assets owned by SHS.

Anything under `.shs/` is owned by SHS Panel and will not be touched by
upstream merges.
