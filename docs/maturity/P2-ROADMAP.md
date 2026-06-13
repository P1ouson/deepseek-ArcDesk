# P2 — Differentiation Roadmap

| # | Capability | Status | Tools |
|---|------------|--------|-------|
| 13 | Runtime RAG | ✅ | `runtime_find` (keyword search on session hub) |
| 14 | UI-aware RAG | ✅ | `ui_status`, `ui_list`, `ui_find`, `ui_read` |
| 15 | Architecture Guardian | ✅ | `architecture_guardian_*` + write gate |
| 16 | Task graph / DAG | ✅ | `taskdag_status`, `taskdag_load`, `taskdag_ready`, `taskdag_start`, `taskdag_complete` |
| 17 | Cost router | ✅ | `cost_router_status`, `cost_router_classify` + compact/subagent routing |
| 18 | Context compression | ✅ | `context_compression_status` + tool-output cap |

## Deprioritized

| # | Capability | Status |
|---|------------|--------|
| 19 | Embedding / vector RAG | ❌ (by design) |

## Dogfood prompts (copy into ArcDesk on a Wails/frontend project)

1. **Runtime RAG** — run app, trigger a console error, then: *Search runtime observations for the error keyword with runtime_find.*
2. **UI RAG** — *List UI components with ui_list, find the chat panel with ui_find, read it with ui_read.*
3. **Guardian** — *Run architecture_guardian_check on a file that violates SPEC (e.g. Wails bind outside desktop/).*
4. **Task DAG** — *Load a 3-step DAG with taskdag_load, complete tasks in dependency order.*
5. **Cost router** — *Call cost_router_classify on a short question vs an explore prompt.*
6. **Context compression** — *Call context_compression_status; confirm tool_output_max_bytes.*

Restart ArcDesk after upgrading so Go-side tools reload.

See [P2-CHECKLIST.md](./P2-CHECKLIST.md) for boot smoke command and copy-paste dogfood script.
