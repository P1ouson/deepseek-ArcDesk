# ArcDesk Dependency Graph — Design (v0.1)

Module-level dependency analysis for ArcDesk: package import graphs, manifest
dependencies, impact analysis, and cycle detection. Complements
`internal/codegraph/` (symbol/call graph via external MCP).

See sections **0.1** and **0.2** below for serialization and concurrency details.

---

## 0.1 index.json serialization structure

### Design decisions

| Topic | Decision |
|-------|----------|
| NodeID on wire | JSON string, **realm prefix preserved** (`"go:arcdesk/internal/agent"`) |
| Map keys (`Out`/`In`/`Impact`/`Nodes`) | JSON object keys = NodeID strings |
| Impact / Cycles / Conflicts | **Persisted** in `index.json` (avoid recompute on every load) |
| `sync.RWMutex` | **Not serialized** — lives on `Index`, not `Graph` |
| `Index.ready` / `sync.Once` | **Not serialized** — runtime only |

### Wire format (`index.json`)

Top-level object maps 1:1 to `indexSnapshot` (internal) / exported `Graph` fields:

```json
{
  "version": 1,
  "root": "/abs/path/to/workspace",
  "builtAt": "2026-06-13T12:00:00Z",
  "buildMethod": "go_list",
  "nodes": {
    "go:arcdesk/internal/agent": {
      "id": "go:arcdesk/internal/agent",
      "kind": "internal_go",
      "name": "arcdesk/internal/agent",
      "dir": "internal/agent",
      "manifest": { "path": "go.mod", "section": "module" },
      "files": ["internal/agent/agent.go"],
      "meta": { "warnings": [] }
    }
  },
  "out": {
    "go:arcdesk/internal/agent": ["go:arcdesk/internal/evidence", "gomod:std:fmt"]
  },
  "in": {
    "go:arcdesk/internal/evidence": ["go:arcdesk/internal/agent"]
  },
  "impact": {
    "go:arcdesk/internal/agent": {
      "direct": ["go:arcdesk/internal/control"],
      "transitive": [["go:arcdesk/cmd/arcdesk"]],
      "external": ["gomod:github.com/BurntSushi/toml"]
    }
  },
  "cycles": [],
  "conflicts": [],
  "files": {
    "internal/agent/agent.go": "go:arcdesk/internal/agent"
  },
  "orphans": [],
  "parseErrors": [
    { "file": "internal/broken.go", "line": 12, "message": "expected declaration" }
  ],
  "stats": {
    "nodeCount": 247,
    "edgeCount": 891,
    "goPackages": 180,
    "jsPackages": 12,
    "parseErrorCount": 1,
    "buildDurationMs": 2100
  }
}
```

### Go structs (json tags)

```go
// indexSnapshot is the on-disk shape for index.json. Graph is rebuilt from this.
type indexSnapshot struct {
    Version     int                        `json:"version"`
    Root        string                     `json:"root"`
    BuiltAt     time.Time                  `json:"builtAt"`
    BuildMethod string                     `json:"buildMethod,omitempty"` // go_list | parser_fallback | merged
    Nodes       map[string]*Node           `json:"nodes"`
    Out         map[string][]string        `json:"out"`
    In          map[string][]string        `json:"in"`
    Impact      map[string]ImpactLayers    `json:"impact,omitempty"`
    Cycles      []Cycle                    `json:"cycles,omitempty"`
    Conflicts   []VersionConflict          `json:"conflicts,omitempty"`
    Files       map[string]string          `json:"files,omitempty"`
    Orphans     []string                   `json:"orphans,omitempty"`
    ParseErrors []ParseError               `json:"parseErrors,omitempty"`
    Stats       Stats                      `json:"stats"`
}

type Meta struct {
    GeneratedAt  time.Time `json:"generatedAt"`
    GitHead      string    `json:"gitHead,omitempty"`
    Fingerprint  string    `json:"fingerprint"`
    IndexVersion int       `json:"indexVersion"`
}
```

**In-memory only (not in index.json)**

| Field | Owner |
|-------|-------|
| `sync.RWMutex` | `Index.mu` |
| `sync.Once` | `Index.once` |
| `atomic.Bool` loading flag (optional) | `Index` |
| Query caches (BuildFailureContext TTL) | `Index` or caller |

**NodeID serialization**

- Type: `type NodeID string`
- JSON: always `"<realm>:<path>"` — same as `NodeID.String()`
- Deserialization: `ParseNodeID(s)` validates realm + non-empty path
- Map keys: Go `encoding/json` marshals `map[NodeID]T` as JSON objects with string keys

**Impact persistence rationale**

- Recomputing impact for ~500 nodes is ~100ms but load-time work adds latency on every tab open
- Persisting keeps `Open()` → first query at O(1) without a rebuild pass
- Full rebuild still recomputes impact before save

**Size estimate (500 nodes)**

| Component | Estimate |
|-----------|----------|
| Nodes (~500 × ~200 B) | ~100 KB |
| Out/In edges (~2000 × ~40 B) | ~80 KB |
| Impact precompute (~500 × ~500 B avg) | ~250 KB |
| Files map (~2000 files × ~60 B) | ~120 KB |
| Overhead (JSON formatting) | ~30% |
| **Total** | **~700 KB – 1.2 MB** typical, **< 2 MB** worst case |

---

## 0.2 Concurrency safety model

### Lock owner

`Index` holds `sync.RWMutex mu`. **`Graph` has no mutex** — it is immutable from the caller's perspective while published under read lock.

```go
type Index struct {
    mu    sync.RWMutex
    root  string
    graph *Graph
    meta  *Meta
    once  sync.Once
}
```

### Lock rules

| Operation | Lock | Notes |
|-----------|------|-------|
| `ImportsOf`, `ImportedBy`, `AffectedBy`, `FindCycles`, `VersionConflicts`, `Status`, `ResolveID` | **RLock** | Read published `graph` pointer |
| `BuildFailureContext` | **RLock** | Read-only queries |
| `RefreshIfStale`, `InvalidateFiles`, initial `buildAndSave` inside `Open` | **Lock** (write) | Replace `graph` + `meta`, persist |
| `SaveIndex` / `LoadIndex` | **No Index lock** | Called only while caller holds write lock, or before Index is published |

### Blocking during refresh

- `RefreshIfStale` holds **write lock** for the entire rebuild + persist (target **< 5s** for ~500 packages)
- Concurrent read queries **block** until refresh completes — acceptable per product decision
- No stale reads: readers always see the last fully built graph

### Open / double-build prevention

```go
func Open(root string) (*Index, error) {
    idx := &Index{root: root}
    idx.once.Do(func() {
        // try LoadIndex; on miss/stale caller may Refresh later
        idx.initErr = idx.loadOrMarkEmpty()
    })
    return idx, idx.initErr
}

func (i *Index) EnsureReady(ctx context.Context) error {
    // sync.Once inside EnsureReady OR reuse Open's once — guarantees single cold init
}
```

- **`sync.Once` on Index**: prevents duplicate cold `LoadIndex` from concurrent `Open()` callers
- **Refresh coalescing**: per-workspace `TryLock` pattern (like repomap) — concurrent `RefreshIfStale` skips if another refresh holds write lock (second caller returns nil immediately OR waits — **Phase 1: skip with TryLock on a refresh mutex nested under write path**)

Recommended Phase 1 refresh pattern:

```go
var refreshLocks sync.Map // workspace -> *sync.Mutex

func (i *Index) RefreshIfStale(ctx context.Context) error {
    mu := lockFor(i.root)
    if !mu.TryLock() { return nil } // another refresh running
    defer mu.Unlock()

    i.mu.Lock()
    defer i.mu.Unlock()
    if !CheckStale(i.root, i.meta) { return nil }
    return i.rebuildLocked(ctx)
}
```

### InvalidateFiles during refresh

- If `RefreshIfStale` is in progress (`TryLock` failed): `InvalidateFiles` sets **`pendingStale` atomic flag**; refresh completes current build; next refresh sees flag and rebuilds again — OR `InvalidateFiles` returns **`ErrRefreshInProgress`** (Phase 1: **set meta stale + return nil**, next explicit refresh picks it up)
- **Phase 1 simplification**: `InvalidateFiles` only sets `meta.fingerprint = ""` or `forceStale` bool under write lock; does not start rebuild inline

### Atomic persist

```
write index.json.tmp + meta.json.tmp
os.Rename(tmp, final)   // atomic on same filesystem
```

- Write lock held during persist — no concurrent readers see half-written graph in memory; on-disk readers always see previous complete generation until rename
- Crash between tmp write and rename: previous `index.json` remains valid; next boot treats missing/corrupt tmp as stale

### Sentinel errors (query when unavailable)

```go
var (
    ErrIndexNotFound   = errors.New("dependency index not found")
    ErrIndexNotReady   = errors.New("dependency index not ready")
    ErrNodeNotFound    = errors.New("dependency node not found")
    ErrRefreshRunning  = errors.New("dependency refresh in progress")
)
```

---

## Confirmed decisions (D1–D8 + extras)

See implementation plan in project chat. Key: JSON persistence, go list primary, Phase 1 full impact recompute, no prompt compose, `[dependency]` config section.
