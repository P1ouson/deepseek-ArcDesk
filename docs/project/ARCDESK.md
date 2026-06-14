# ARCDESK project memory

This file is loaded into every session's system prompt (the cache-stable prefix),
so keep it concise and durable — it is the project's standing instructions to the
agent. It is the ARCDESK analog of Claude Code's CLAUDE.md.

## Conventions

- Go kernel under `internal/`; each package owns one concern and documents it in a
  package comment. Match the surrounding comment density and idiom when editing.
- One transport-agnostic `control.Controller` sits behind every frontend (chat
  TUI, HTTP/SSE serve, Wails desktop). Add behavior to the controller, not a
  frontend, so all three inherit it.
- Cache-first: the system-prompt prefix (base prompt + tools + memory) must stay
  byte-stable across turns so DeepSeek's automatic prefix cache stays warm. Never
  mutate it mid-session — ride the turn tail instead (see `control.Compose`).

## Memory

- Hierarchical docs: `ARCDESK.md` (this file, committed/shared), `ARCDESK.local.md`
  (personal, git-ignored), user-global `~/.config/ARCDESK/ARCDESK.md`, and any
  `ARCDESK.md` in an ancestor dir. `AGENTS.md` is accepted as a fallback name.
- `@path` on its own line imports another file's contents.
- `#<note>` in chat quick-adds a line here. The `remember` tool saves durable
  facts to the per-project auto-memory store (frontmatter files + `MEMORY.md`
  index), which loads into the prefix on the next session.

## Notes

### Frontend design stack (always on)

Before editing `desktop/frontend` UI/CSS:

1. **Impeccable** — `.ARCDESK/skills/impeccable/` (`run_skill` / `/impeccable`); read `docs/PRODUCT.md`, `docs/DESIGN.md`, `reference/product.md`
2. **taste-skill-desktop** — `.cursor/rules/taste-skill-desktop.mdc`
3. **ui-flat-containers** — max 2 bordered/radius layers
4. **Karpathy** — minimal diff, verify with tsc

Cursor mirrors: `.cursor/skills/impeccable/`, `.cursor/rules/impeccable-always.mdc`
