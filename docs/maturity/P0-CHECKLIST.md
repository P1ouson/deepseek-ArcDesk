# P0 — Mature Coding Agent Checklist

Track P0 completion as **code + dogfood**. Code alone is not enough for ✅.

Legend: ✅ done · ⚠️ partial · ❌ missing

| # | Capability | Package / tools | Code | Dogfood |
|---|------------|-----------------|------|---------|
| 1 | Repo-aware RAG | `reporag` → `repo_*`; `dependency_*`; CodeGraph MCP; `repomap` | ⚠️ | ☐ |
| 1a | Symbol graph | `codegraph_*`, `callgraph_trace_*`, `repo_symbol` | ⚠️ | ☐ |
| 1b | Dependency graph | `dependency_*` | ✅ | ☐ |
| 1c | Code navigation | `repo_navigate`, `lsp_*` | ⚠️ | ☐ |
| 2 | Call graph | `callgraph_*`, `callgraph_breakpoints` | ✅ | ☐ |
| 3 | Runtime observation | `runtime_*`, frontend `runtimeObserve.ts` | ⚠️ | ☐ |
| 4 | Self-debug loop | `selfdebug_*`, agent retries | ✅ | ☐ |
| 5 | Verification engine | `verification_*`, discover + `after_write` | ⚠️ | ☐ |
| 6 | Constraint system | `constraint_*` | ✅ | ☐ |
| 7 | Rollback / checkpoint | `checkpoint`, `rollback_*`, `on_failure=rollback` | ✅ | ☐ |

## Dogfood script (~15 min)

Run from repo root with `arcdesk.toml` present.

1. **Boot** — start desktop or `arcdesk serve`; confirm no boot errors for callgraph/runtime/codegraph.
2. **Repo RAG** — ask agent to run `repo_status`; expect dependency + callgraph layers ready on this repo.
3. **Symbol** — `repo_symbol` query e.g. `Build` or `Controller`; CodeGraph may warm in background first session.
4. **Call graph** — `callgraph_trace_forward` from a UI handler; `callgraph_breakpoints` on changed paths.
5. **Runtime** — open app tab; trigger console log; `runtime_tail` with `kind=console`.
6. **Write loop** — small edit under `internal/`; confirm `go build` + `go test ./internal/...` run (verification).
7. **Failure** — intentionally break a test; confirm self-debug hint and retry (max 3).
8. **Rollback** — exhaust retries; with `on_failure=rollback`, workspace should rewind (controller auto-rewind).

## Exit criteria (P0 complete)

- All rows in table marked **Code ✅** where applicable, **Dogfood ✅** after script above.
- `go test ./internal/boot/... -run TestBuildRegistersP0Tools` passes.
- `go test ./internal/config/... -run TestDogfood` passes.
- No P0 tool missing from boot on this repository.

## Config reference

Project dogfood settings: [`arcdesk.toml`](../../arcdesk.toml).

## Next

When P0 dogfood is green, proceed to [P1-CHECKLIST.md](./P1-CHECKLIST.md) (experience polish), then P2.
