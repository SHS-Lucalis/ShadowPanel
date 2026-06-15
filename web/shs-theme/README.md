# `web/shs-theme/` — SHS Panel dark theme (frontend)

> Status: **scaffold (Phase 0)**. Theme work lands in Phase 3.

This package owns the Pelican-inspired modern dark UI described in
`docs/architecture/00-shs-panel-architecture-plan.md` §9.

Contents (to be added in Phase 3):

- `tailwind.config.shs.cjs` — SHS Tailwind preset (dark tokens, brand
  colours, typography scale).
- `components/` — shadcn-vue component overrides and SHS-specific
  composites (server card, console card, variable form generator, etc.).
- `tokens.css` — CSS variables for both dark (default) and light themes.

Migration model:

- Upstream `web/frontend/` keeps its existing Naive UI components.
- The new theme is gated behind the `SHS_THEME=true` env flag during
  rollout so we can ship page-by-page (login → server list → server
  detail → console → file manager → variable editor → admin).
- Default flips to `SHS_THEME=true` once UI parity is reached.
