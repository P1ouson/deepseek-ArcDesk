---
theme: Editorial Studio
register: product
palette:
  bg: "#fbfbfa"
  bgSoft: "#f7f6f3"
  bgElev: "#ffffff"
  ink: "#2f3437"
  inkDim: "#787774"
  inkFaint: "#9b9a97"
  accent: "#5e6ad2"
  accentSoft: "rgba(94, 106, 210, 0.12)"
  border: "rgba(55, 53, 47, 0.11)"
  borderSoft: "rgba(55, 53, 47, 0.07)"
  ok: "#346538"
  warn: "#956400"
  err: "#9f2f2d"
typography:
  sans: "Segoe UI Variable, Segoe UI, PingFang SC, Noto Sans SC, sans-serif"
  display: "Segoe UI Variable Display, Segoe UI Variable, Segoe UI, sans-serif"
  mono: "Cascadia Mono, SF Mono, ui-monospace, Consolas, monospace"
  uiSize: 12px
  metaSize: 11px
  titleSize: 15px
radii:
  sm: 4px
  md: 6px
  lg: 8px
motion:
  duration: 140ms-200ms
  easing: ease-out
  reducedMotion: required
---

# Design system — ARCDESK Desktop

## Atmosphere

Warm editorial monochrome devtools shell. Single restrained indigo accent. Flat surfaces, hairline dividers, minimal shadow (overlays only).

## Color rules

- Body text on `--bg-elev` / `--chat-bg`: use `--fg` (#2f3437), not faint gray alone.
- Accent (`--accent`) for primary actions, active tabs, selection only.
- Semantic pastels for status pills (git/modified/added/deleted), low saturation.
- **Never** purple-to-blue marketing gradients or saturated hero backgrounds.

## Typography

- Panel titles: `--display`, 15px, weight 620–650, letter-spacing -0.02em.
- Paths / counts / commands: `--mono`, 10–11px, `--fg-faint`.
- List labels: 12px semibold; secondary detail 10–10.5px.
- Section eyebrows: 11px uppercase + letter-spacing, sparingly.

## Layout

- Workbench: sidebar | main (stack + optional preview + dock + tool rail).
- Right dock panels: `dock-panel` + modifier; head with bottom border only.
- Lists: divider rows, not per-row cards.
- Max **two** bordered/radius layers per chain (see `.cursor/rules/ui-flat-containers.mdc`).

## Components

### dock-panel

Shared right-dock chrome: head / filter / scroll body / footer. Ghost icon buttons; underline filter input.

### write-studio

Writing mode: sidebar file list + editor/preview + assistant column. Segmented view control in single outer frame.

### composer-shell

One bordered card at footer; inner controls borderless. Shadow: `--shadow` only, not heavy float.

### Status pills

Pastel background + semantic foreground; pill radius `--radius-pill`.

## Motion

150–200ms transitions on hover/focus. No page-load choreography. `@media (prefers-reduced-motion: reduce)` required.

## Named rules

- **NO_NESTED_CARDS**: Never stack three bordered rounded containers.
- **PRESERVE_TOKENS**: Do not replace `:root` palette on panel work.
- **ACCENT_SPARING**: One accent hue; no decorative color blocks.
- **FLAT_COMPOSER**: Composer uses one shell border; inner fields borderless or single underline.
