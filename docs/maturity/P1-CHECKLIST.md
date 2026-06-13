# P1 — Strongly Recommended Checklist

| # | Capability | Package / tools | Code | Dogfood |
|---|------------|-----------------|------|---------|
| 8 | Git-aware RAG | `gitrag` → `git_*` | ✅ | ☐ |
| 9 | Architecture RAG | `archrag` → `architecture_*` | ✅ | ☐ |
| 10 | Real planner | `planner` → `planner_*`, coordinator gates | ✅ | ☐ |
| 11 | Failure memory | `failuremem` → `failuremem_*` | ✅ | ☐ |
| 12 | Environment awareness | `envaware` → `environment_*` | ✅ | ☐ |

Test coverage: all P1 packages ≥ 90%.

Dogfood: enable in `arcdesk.toml` (defaults on when git repo / docs present). Verify tools register after boot.

Proceed to P2 only after P0 + P1 dogfood pass.
