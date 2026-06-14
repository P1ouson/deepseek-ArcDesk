# ARCDESK Desktop — Product context

## Register

product

## Target users

Developers using ARCDESK as a local AI coding workbench: chat, file preview, Git, terminal, writing mode, and MCP/skills integration.

## Product purpose

A focused devtools shell where the agent and the user co-edit a workspace. Design should disappear into the task (Linear / VS Code familiarity), not compete with the code.

## Brand personality

Specific, restrained, trustworthy, editorial. Warm monochrome surfaces with one indigo accent. Cockpit density (4–5), low motion (2–4).

## Anti-references

- Marketing landing pages (hero + three feature cards + purple gradient)
- Nested rounded cards (3+ bordered layers)
- Heavy drop shadows and glassmorphism chrome
- AI-purple gradients and generic Inter-only typography resets
- Replacing the whole theme on every panel tweak

## Strategic design principles

1. **Redesign-preserve** — extend existing tokens (`--fg`, `--accent`, `--display`, `--mono`) and shared classes (`dock-panel__*`, `write-studio__*`).
2. **Flat hierarchy** — dividers and row hover beat card stacks; at most two bordered/radius containers per visual chain.
3. **Product register** — accent only for primary actions, selection, and semantic state; not decoration.
4. **Editorial devtools** — display font for panel titles only; mono for paths, counts, commands; 11–13px UI density.
5. **Verify before ship** — contrast ≥4.5:1 for body text; focus rings on interactive controls; `pnpm exec tsc --noEmit` for frontend changes.

## Accessibility

Keyboard navigation and visible focus required. Respect `prefers-reduced-motion`. Sufficient contrast on muted text over tinted backgrounds.
