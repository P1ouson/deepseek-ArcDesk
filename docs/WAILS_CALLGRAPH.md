# ArcDesk Wails Call Graph — Design (v0.1)

**Status:** Implemented in `internal/callgraph/` (v0.1). Design doc kept for architecture reference.

---

## 总体架构概览

Wails Call Graph（`internal/callgraph/`）是 ArcDesk 的**跨 realm 静态调用链索引**：在 React/TS 前端、Wails bridge、`desktop.App` Go 绑定方法、以及 Go 内部符号调用之间建立可查询的前向/后向路径。它与 **Dependency Graph**（模块级：哪个包依赖哪个包）和 **CodeGraph**（符号级：Go 函数谁调谁）互补，专门回答「这个 Button 点了之后 Go 端跑了哪些代码」以及「改了 `GetUser` 的返回结构，前端哪些 hook/组件要改」。

**核心设计原则：**

1. **Bridge 层是一等公民** — 索引以 `wailsjs/go/main/App.d.ts` + `bridge.ts` 的 `app.*` 调用为前端锚点，以 `func (a *App) Method(...)` 为 Go 锚点。
2. **静态分析 only** — 不依赖运行时 trace；Events 通道用命名约定 + 源码模式匹配。
3. **复用不耦合** — 通过小接口消费 Dependency / CodeGraph；任一不可用时有降级。
4. **高缓存命中** — 全量预计算 bridge 映射 + 邻接表；查询 O(1) 查表 + O(k) 路径展开（k = 路径长度，通常 ≤ 8）。

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Agent Tools (readonly)                          │
│   callgraph_trace_forward │ callgraph_trace_backward │ callgraph_find_bridge │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │ Index API (RLock)
┌───────────────────────────────────▼─────────────────────────────────────┐
│                         internal/callgraph/                           │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────┐  ┌────────────┐ │
│  │ TS/React    │  │ Bridge       │  │ Go Bind     │  │ Go Internal│ │
│  │ Indexer     │→ │ Resolver     │→ │ Indexer     │→ │ (optional) │ │
│  └─────────────┘  └──────────────┘  └─────────────┘  └─────┬──────┘ │
│         │                  │                  │              │        │
│         └──────────────────┴──────────────────┴──────────────┘        │
│                              CallGraph (immutable snapshot)            │
│   nodes │ edges │ bridgeMap │ eventMap │ forward/out │ backward/in    │
└───────────────┬───────────────────────────────┬───────────────────────┘
                │                               │
     ┌──────────▼──────────┐         ┌──────────▼──────────┐
     │ dependency.Module   │         │ codegraph.Symbol    │
     │ Catalog (interface) │         │ Query (interface)   │
     │ go/js package IDs   │         │ callers/callees     │
     └─────────────────────┘         └─────────────────────┘
                │                               │
     .arcdesk/dependency/              .codegraph/ (MCP, optional)
     .arcdesk/callgraph/  ← JSON + meta.json (fingerprint)
```

**与现有系统的关系（一句话）：**

| 系统 | 粒度 | Call Graph 如何使用 |
|------|------|---------------------|
| Dependency Graph | 包/模块 | 文件→包归属、Go/JS 分类、bridge 包定位 |
| CodeGraph | Go 符号 | Go bind 方法以下的 callees/impact（可选） |
| Wails bind | 方法名 | `App.Method` ↔ `app.Method()` 映射的核心 |

---

## 1. 核心概念定义

### 1.1 节点类型（NodeKind）

| Kind | 说明 | 示例 ID | 解析来源 |
|------|------|---------|----------|
| `ui_component` | React 函数/类组件 | `ui:desktop/frontend/src/components/FilesPanel.tsx#FilesPanel` | TSX export default / named export + 文件名 |
| `ui_handler` | 组件内事件处理、callback | `ui:.../Composer.tsx#handleSubmit` | JSX `onClick={...}` / `useCallback` 命名函数 |
| `hook` | 自定义 hook 定义 | `hook:desktop/frontend/src/lib/useDesktopSendRouter.ts#useDesktopSendRouter` | `function useXxx(` / `export function useXxx` |
| `ts_function` | 普通 TS 函数（非 hook） | `fn:desktop/frontend/src/lib/bridge.ts#submitWithGate` | 具名函数声明 |
| `bridge_call` | 前端调用 Wails bind 的站点 | `bridge:desktop/frontend/src/lib/useController.ts:142#app.Submit` | `app.Method(` / `window.go.main.App.Method(` |
| `wails_export` | wailsjs 生成的导出符号（逻辑节点） | `wexport:go/main/App#Submit` | `App.d.ts` / `App.js` |
| `go_bind` | Go 端暴露给前端的 bind 方法 | `gobind:desktop/app.go#App.Submit` | `func (a *App) Submit(` |
| `go_internal` | Go 内部符号（非 bind） | `go:desktop/tabs.go#tabEventSink.Emit` | Go AST / CodeGraph |
| `event_emit` | Go 端 EventsEmit 站点 | `emit:desktop/tabs.go:115#agent:event` | `runtime.EventsEmit(ctx, "channel", ...)` |
| `event_listen` | 前端 EventsOn 订阅站点 | `listen:desktop/frontend/src/lib/bridge.ts:386#agent:event` | `EventsOn("channel", ...)` / `onEvent(` 包装 |

**决策理由：**

- **`ui_handler` 与 `ui_component` 分离**：组件级影响面分析需要 handler 粒度；展示时可折叠到组件。
- **`bridge_call` 与 `wails_export` 分离**：ArcDesk 大量通过 `app` Proxy 调用，不直接 import wailsjs；bridge_call 是真实调用点，wexport 是命名规范锚点。
- **`go_bind` 与 `go_internal` 分离**：前向追踪在 bind 方法处必须停止展开到「对外 API 边界」，内部链路由 CodeGraph 按需延伸。

**替代方案：**

| 方案 | 优点 | 缺点 | 结论 |
|------|------|------|------|
| A. 仅 bridge_call + go_bind 两种节点 | 简单 | 无法回答「哪个组件」 | ❌ 不满足需求 |
| B. 把 hook 合并进 ts_function | 少一种 kind | 影响面分析难区分 hook | ❌ |
| **C. 上表 10 种（Phase 1 用 8 种，event 2 种 Phase 1 简版）** | 覆盖 UI→Go 和 Go→UI | 索引稍大 | ✅ 推荐 |

Phase 1 可先不建独立 `wails_export` 节点，仅在 `bridgeMap` 中存 `(package, method) → gobind`；event 节点用边属性携带 channel 名。

### 1.2 边类型（EdgeKind）

| Kind | 方向 | 含义 |
|------|------|------|
| `calls` | A → B | 函数/handler 调用函数/hook/bridge |
| `bridge_invoke` | bridge_call → go_bind | 跨 realm：TS 调用 Go bind |
| `go_calls` | go_bind → go_internal | Go 内部调用（可来自 CodeGraph） |
| `emits` | go_internal/go_bind → event_emit | Go 发出事件 |
| `listens` | event_listen → ui_handler/ui_component | 前端订阅并触发逻辑 |
| `event_delivers` | event_emit → event_listen | 同 channel 名的逻辑边（非 AST 边） |
| `renders` | ui_component → ui_component | JSX 子组件（可选 Phase 2，用于 props 追踪） |
| `hook_used_by` | hook → ui_component | 组件使用 hook（`useXxx()` 在组件内） |

**决策理由：**

- **`bridge_invoke` 单独成边**：跨 realm 边是查询的核心，需 O(1) 索引。
- **`event_delivers` 是推断边**：静态连接 `EventsEmit("agent:event")` 与 `EventsOn("agent:event")`；不保证运行时一定触发。
- **`renders` Phase 2**：props 传递分析复杂度高，Phase 1 不做。

### 1.3 待确认

- [ ] Phase 1 是否包含 `event_emit` / `event_listen` / `event_delivers`，还是仅 RPC（bridge_invoke）？
- [ ] `ui_handler` 是否要支持匿名箭头函数（仅记录为 `ui:...#anonymous:line42`）？
- [ ] 是否追踪 `terminalBridge.ts` 等二级 bridge 包装（`app.StartTerminal` 外包一层）？

---

## 2. 包结构设计

```
internal/callgraph/
├── doc.go                 # 包文档；与 dependency/codegraph 的分工说明
├── types.go               # Node, Edge, CallPath, NodeKind, EdgeKind — exported
├── id.go                  # NodeID 解析/构造 — exported
├── graph.go               # CallGraph 内存模型 + 邻接表 — exported
├── index.go               # Index 生命周期 + 查询 API — exported
├── store.go               # index.json / meta.json 持久化 — exported
├── meta.go                # fingerprint, CheckStale — exported
├── refresh.go             # RefreshIfStale, InvalidateFiles — exported
├── build.go               # BuildGraph 编排 — exported
├── catalog.go             # ModuleCatalog interface + 适配 dependency — exported
├── symbol.go              # SymbolQuery interface + MCP/noop 适配 — exported
├── tsindex.go             # TS/TSX 解析（组件/hook/bridge 调用）— internal
├── gobind.go              # Go bind 方法提取（desktop/*.go）— internal
├── bridge.go              # bridge 映射表构建 — internal
├── events.go              # EventsEmit/EventsOn 静态匹配 — internal
├── paths.go               # CallPath 组装、截断、格式化 — exported
├── format.go              # Agent/LLM 输出格式 — exported
├── tools.go               # RegisterTools — exported
├── context.go             # BuildCrossRealmContext（verify/编辑提示）— exported
├── discover.go              # Wails 项目探测（wailsjs 存在、desktop/main.go Bind）— exported
└── *_test.go
```

### 2.1 Exported vs internal

| Exported（其他包可 import） | Internal（实现细节） |
|---------------------------|----------------------|
| `Open`, `Index`, `BuildGraph`, `RegisterTools` | TS AST 遍历细节 |
| `types`, `NodeID`, `CallPath` | bridge 正则/heuristics |
| `ModuleCatalog`, `SymbolQuery` 接口 | MCP JSON 协议细节 |
| `BuildCrossRealmContext` | |

**原则：** 与 `dependency` 一致 — 外部只通过 `Index` + tools 访问；图本身 immutable + RWMutex。

### 2.2 与 `internal/dependency/` 的边界

```go
// callgraph/catalog.go — callgraph 定义，dependency 实现（adapter 在 boot 注入）

type ModuleCatalog interface {
    // ResolveFile 返回文件所属模块 NodeID（go:... 或 js:...）
    ResolveFile(relPath string) (moduleID string, ok bool)
    // IsGoModule / IsJSModule 用于过滤
    ModuleKind(moduleID string) (kind string, ok bool)
    // BridgePackage 返回 wailsjs 所在 JS 包 ID（如 js:desktop/frontend/wailsjs/go/main）
    BridgeRoot() (jsModuleID string, ok bool)
}
```

- **callgraph 不 import dependency 的具体类型**（不依赖 `dependency.NodeID`），只用 string 模块 ID，避免循环依赖。
- boot 层：`callgraph.NewDependencyCatalog(depIdx)` 适配器。

**替代方案：**

| 方案 | 结论 |
|------|------|
| callgraph 直接 import dependency.Index | 简单但耦合；dependency 将来 import callgraph 会循环 | ❌ |
| **interface + boot adapter** | 与 CodeGraph 模式一致 | ✅ |
| 重复解析 go.mod/package.json | 重复建设 | ❌ |

### 2.3 与 CodeGraph 的关系

CodeGraph 在 ArcDesk 中是 **MCP 外部进程**，无 Go 内存 API。

```go
// callgraph/symbol.go

type SymbolQuery interface {
    Available() bool
    Callers(symbol string) ([]SymbolRef, error)  // 可选，Phase 1 可 stub
    Callees(symbol string) ([]SymbolRef, error)
}

// 实现：
// - mcpSymbolQuery：通过已连接的 MCP client 调 codegraph_callees（boot 注入）
// - noopSymbolQuery：Available()=false，Build 时跳过 go_internal 扩展
// - astSymbolQuery（降级）：仅解析 desktop/*.go 同文件 callees，深度=1
```

**决策：** Phase 1 **预计算 go_bind → 同文件 callees（AST 深度 1）**；CodeGraph 作为 **查询时按需扩展**（trace 工具参数 `depth`/`include_go_internal`），不阻塞索引构建。

### 2.4 待确认

- [ ] 包名 `callgraph` vs `wailscgraph` vs `crossrealm`？
- [ ] `ModuleCatalog` 是否还需要 `ListJSFiles()` 或依赖 callgraph 自己 walk？
- [ ] CodeGraph MCP 适配放在 `callgraph` 还是 `internal/codegraph` 里做 adapter？

---

## 3. 核心数据结构

### 3.1 NodeID 设计

沿用 Dependency Graph 的 **realm 前缀** 风格，但 realm 不同：

```
<realm>:<repo-relative-path>#<symbol>[::<qualifier>]
```

| Realm | 含义 | 示例 |
|-------|------|------|
| `ui` | React 组件 | `ui:desktop/frontend/src/App.tsx#App` |
| `hook` | Hook | `hook:desktop/frontend/src/lib/useController.ts#useController` |
| `fn` | TS 函数 | `fn:desktop/frontend/src/lib/bridge.ts#mapWireEvent` |
| `bridge` | Bridge 调用点 | `bridge:desktop/frontend/src/components/Composer.tsx:891#app.Submit` |
| `gobind` | Go bind | `gobind:desktop/app.go#App.Submit` |
| `go` | Go 内部 | `go:desktop/tabs.go#(*App).ListTabs` |
| `emit` | EventsEmit | `emit:desktop/tabs.go:119#agent:event` |
| `listen` | EventsOn | `listen:desktop/frontend/src/lib/bridge.ts:388#agent:event` |

**`:line` 后缀**（bridge/emit/listen）：用于同符号多调用点消歧；持久化时保留。

### 3.2 CallPath（调用链）

```go
type CallPath struct {
    ID       string      // 稳定 hash（起止+边序列）
    Direction string     // forward | backward
    Segments []PathSegment
    Truncated bool
    Hint     string      // 如 "via bridge app.Submit"
}

type PathSegment struct {
    Node   NodeSnapshot
    Edge   EdgeKind      // 到达下一节点所用的边
    RealmCross bool      // 是否跨 realm（bridge_invoke / event_delivers）
}

type NodeSnapshot struct {
    ID   NodeID
    Kind NodeKind
    Name string          // 展示名：FilesPanel, app.Submit, App.Submit
    File string
    Line int
}
```

**路径表示例（forward）：**

```
FilesPanel.tsx#onOpen
  →[calls]→ useWorkspaceFiles.ts#openFile
  →[calls]→ bridge: app.ListDir @ useWorkspaceFiles.ts:88
  →[bridge_invoke]→ gobind: App.ListDir @ desktop/app.go
  →[go_calls]→ go: workspace.ListDir (optional, CodeGraph)
```

### 3.3 快速查询索引（预计算）

| 索引 | 结构 | 用途 |
|------|------|------|
| `out` / `in` | `map[NodeID][]Edge` | 通用邻接表 |
| `bridgeByMethod` | `map[string][]NodeID` | key=`Submit` → 所有 bridge_call 节点 |
| `goBindByMethod` | `map[string]NodeID` | key=`Submit` → gobind 节点（多 receiver 时 `App.Submit` 全名） |
| `methodMap` | `map[string]NodeID` | `App.Submit` → gobind（前端 method 名 → Go） |
| `eventByChannel` | `map[string]{emit[], listen[]}` | 事件通道索引 |
| `fileIndex` | `map[string][]NodeID` | 文件 → 节点，增量失效用 |
| `componentHooks` | `map[NodeID][]NodeID` | 组件 → 使用的 hooks |

**查询复杂度：**

- `FindBridge("Submit")` → O(1) `bridgeByMethod`
- `TraceForward(from)` → BFS/Dijkstra on out edges，O(V+E) 单次；ArcDesk 规模 V~2k E~8k，<1ms
- `TraceBackward(to)` → BFS on in edges

### 3.4 待确认

- [ ] NodeID 是否要与 dependency.NodeID 共用解析器（共享 `internal/realmid` 包）？
- [ ] CallPath 最大深度默认 12 还是 8（与 Dependency Impact 对齐）？
- [ ] 是否需要路径去重 canonical form（同一路径只存 hash）？

---

## 4. Bridge 解析策略

> ArcDesk 特有问题：**前端不直接 import wailsjs**，而是通过 `bridge.ts` 的 `app` Proxy。

### 4.1 识别前端 bridge 调用

**Tier 1 — 高置信（Phase 1 必须）：**

| 模式 | 示例 | 提取 |
|------|------|------|
| `app.<Method>(` | `await app.Submit(payload)` | method=`Submit` |
| `app?.<Method>(` | optional chaining | 同上 |
| `window.go.main.App.<Method>(` | 直接绑定 | method |
| import wailsjs + call | `import { Submit } from '../../wailsjs/go/main/App'` | method |

**Tier 2 — 中置信：**

| 模式 | 处理 |
|------|------|
| `const { Submit } = app` 后再 `Submit()` | 数据流追踪（Phase 2） |
| 动态 `app[methodName]()` | 记录为 `bridge:dynamic` + warning |

**解析器实现：** 复用 dependency 的 TS 解析基础设施（`jsparse.go` 同源思路），在 callgraph 内用 **regex + 括号深度** 做 Phase 1（与 dependency JS 索引一致，零新依赖）；Phase 2 可选 `go/parser` 等价物不现实，TS 需轻量 AST 或继续 heuristic。

**文件优先级：**

1. `desktop/frontend/wailsjs/go/main/App.d.ts` — **权威方法名列表**（~130 方法）
2. `desktop/frontend/src/lib/bridge.ts` — `AppBindings` interface 交叉验证
3. `scripts/check-bridge-drift.mjs` 逻辑可对齐（扫描 `func (a *App) Method(`）

### 4.2 前端 bridge → Go bind 匹配规则

```
bridge_call.methodName  ──→  gobind: desktop.App.{MethodName}
```

| 规则 | 说明 |
|------|------|
| 方法名精确匹配 | `app.Submit` → `func (a *App) Submit` |
| Receiver 固定 | ArcDesk 仅 bind `*App`（`main.go: Bind: []any{app}`） |
| 大小写敏感 | Go exported 方法 ↔ TS 同名 |
|  overload | Go 无重载；TS 侧不同签名仍映射同一 gobind |

**多 bind struct（未来）：** NodeID 用 `gobind:desktop/app.go#App.Submit`；`methodMap` key 用 `App.Submit`；若增加 `OtherService`，改为 `OtherService.Foo`。

### 4.3 wailsjs/ 自动生成代码的角色

| 文件 | 是否解析 | 用途 |
|------|----------|------|
| `App.d.ts` | ✅ 读方法名+签名 | 权威方法枚举；匹配校验 |
| `App.js` | ⚠️ 可选 | 确认 `window['go']['main']['App']` 路径；Phase 1 不建节点 |
| `models.ts` | ❌ Phase 1 | 返回类型结构分析留 Phase 2（影响面） |
| `runtime/runtime.d.ts` | ✅ | EventsOn/EventsEmit/BrowserOpenURL 等 |

**决策：** wailsjs 是 **匹配依据 + 方法 catalog**，不是主要 call graph 节点来源；真实调用边来自 `src/` 下 TS/TSX。

### 4.4 EventsEmit / EventsOn 追踪

**Go 端提取：**

```go
runtime.EventsEmit(ctx, "agent:event", payload)
//            ^const string literal
```

- 正则 + go/ast：`EventsEmit` 调用第 2 参数为 string literal → 记录 channel
- 已知 channel 表（ArcDesk）：`agent:event`, `agent:ready`, `agent:tabs-shell`, `terminal:output`, `terminal:exit`, `updater:progress`, `project-tree:changed`, `schedule:task`, `mobile:message`, `app:open-settings`, ...

**前端提取：**

- `EventsOn("agent:event", cb)` in runtime imports
- 包装函数：`onEvent()` → 查 `bridge.ts` 内 `EVENT_CHANNEL = "agent:event"` 常量传播

**连接：** `event_delivers` 边，channel 名为 key；**不**尝试证明 callback 内调用了什么（Phase 2 可内联一层）。

### 4.5 待确认

- [ ] Phase 1 是否解析 `AppBindings` interface 作为 method catalog 备份（当 wailsjs 缺失）？
- [ ] `app` Proxy 的 `get` trap 是否导致所有 `app.Xxx` 都需扫描（是）？
- [ ] 是否支持 `SubmitToTab` 等 Tab 变体与 base 方法关联？

---

## 5. 调用链构建算法

### 5.1 前向追踪（UI → Go）

**输入：** `NodeID` 或 `file:line#symbol` 或 shorthand `Composer.tsx#handleSubmit`

**算法：**

```
BFS from start node on `out` edges
  edge priority: calls > hook_used_by > bridge_invoke > go_calls
  stop nodes: go_bind (always include)
  optional extend: go_bind → go_internal (depth N, if SymbolQuery available)
  max depth: 12 (default), max paths: 5
  terminate: queue empty | depth limit | duplicate path hash
```

**一对多：** 返回 `[]CallPath`（最多 5 条），按长度 + 置信度排序。

### 5.2 后向追踪（Go → UI）

**输入：** `gobind:desktop/app.go#App.Notify` 或 method name `Notify`

**算法：**

```
BFS on `in` edges from gobind
  include: go_calls ← gobind ← bridge_invoke ← calls ← hook_used_by ← ui_*
  optional: event_delivers backward from emit nodes
  stop: ui_component / ui_handler
```

**Notify 类方法：** 若 Go 方法主要是 `EventsEmit`，自动 append event 段：

```
App.Notify → emit:agent:event → listen:onEvent → hook/event handler → Component
```

### 5.3 断点定位（find_bridge）

**输入：** 前端 handler ID + 后端 gobind ID（或 method name）

**算法：** 双向 BFS 中间相遇（bi-directional search） on 无向图视图，或从 handler forward + gobind backward 取交集。

### 5.4 展示格式

**Agent tool 输出（结构化 + 短摘要）：**

```
Trace forward from bridge:Composer.tsx:891#app.Submit
Path 1 (4 hops, 1 realm cross):
  Composer.handleSubmit @ Composer.tsx:891
  → useDesktopSendRouter.send @ useDesktopSendRouter.ts:44
  → app.Submit @ useDesktopSendRouter.ts:102 [bridge]
  → App.Submit @ desktop/app.go:210 [go bind]
JSON: [{...PathSegment...}]
```

**LLM context 注入（`BuildCrossRealmContext`）：**

```markdown
## Wails Call Chain
- **UI**: `Composer.tsx` handleSubmit
- **Bridge**: `app.Submit` (2 call sites)
- **Go bind**: `App.Submit` → delegates to `control.Controller.Run`
- **Event back**: `agent:event` → `onEvent` → `App.tsx`
Max 8 lines; no full JSON.
```

### 5.5 待确认

- [ ] 默认返回路径数 3 还是 5？
- [ ] go_internal 默认深度：0（仅 bind）/ 1（AST）/ 3（CodeGraph）？
- [ ] 路径排序：短优先 vs bridge 优先？

---

## 6. 索引构建流程

### 6.1 全量 Pipeline

```
1. Discover          → 是否 Wails 项目（desktop/main.go Bind + frontend/）
2. Load catalogs     → ModuleCatalog (dependency), method list (App.d.ts)
3. Go bind scan      → desktop/**/*.go: func (a *App) Xxx
4. TS/TSX scan       → components, hooks, app.Method calls, EventsOn
5. Go events scan    → EventsEmit string literals
6. Build edges       → calls (intra-TS), bridge_invoke, event_delivers
7. Optional Go AST   → go_bind → local callees (depth 1)
8. Precompute indexes→ bridgeByMethod, eventByChannel, out/in
9. Save              → .arcdesk/callgraph/index.json + meta.json
```

**目标耗时（ArcDesk）：** < 3s — TS scan ~1.5s, Go scan ~0.5s, edges ~0.5s（与 dependency refresh 并行或串行均可）

### 6.2 依赖 Dependency Graph 的信息

| 信息 | 用途 |
|------|------|
| `Files` map（path → module） | 文件归属、过滤 testdata |
| JS 包边界 | 限定 scan 范围在 `desktop/frontend/src` |
| `js:desktop/frontend/wailsjs/...` | 标记 bridge 生成目录 |
| Orphans 无关 | callgraph 自己标记 bridge 节点 |

**不依赖：** impact/cycles/edges 详情。

### 6.3 wailsjs/ 处理

| 场景 | 行为 |
|------|------|
| 存在且新鲜 | 解析 App.d.ts，校验 methodMap |
| 不存在 | WARN + 降级：扫描 `AppBindings` + Go bind 扫描交集 |
| 过期 | fingerprint 含 wailsjs mtime；stale 触发 rebuild |

### 6.4 增量策略（Phase 1 → 1.5）

| 变更 | 动作 |
|------|------|
| TS/TSX 文件 | 标记 fileIndex 中节点 stale；Phase 1 **全量 rebuild**（与 dependency 一致） |
| Go desktop/*.go | 全量 rebuild |
| wailsjs/App.d.ts 变更 | 全量 rebuild + methodMap  diff 日志 |
| 仅 node_modules | 忽略 |

**Phase 1.5 优化：** 按文件增量 re-parse + 局部边更新（`InvalidateFiles` 已有钩子）。

### 6.5 待确认

- [ ] callgraph 构建是否 **依赖 dependency EnsureReady 先完成**（推荐是，boot 串行）？
- [ ] 是否索引 `desktop/` 下全部 Go 还是仅 `app.go` + 绑定相关？
- [ ] test/mock 文件是否 exclude（`**/*.test.ts`, `bridge.ts` mock 段）？

---

## 7. 查询接口设计

### 7.1 Index API（Go 库）

```go
func Open(root string) (*Index, error)
func (i *Index) EnsureReady(ctx context.Context) error
func (i *Index) TraceForward(ctx context.Context, q ForwardQuery) ([]CallPath, error)
func (i *Index) TraceBackward(ctx context.Context, q BackwardQuery) ([]CallPath, error)
func (i *Index) FindBridge(ctx context.Context, q BridgeQuery) (BridgeMatch, error)
func (i *Index) AffectedUI(ctx context.Context, goMethod string) ([]NodeSnapshot, error) // 后向扁平
func (i *Index) Status() (Stats, error)
```

### 7.2 Agent Tools

#### `callgraph_trace_forward`

```json
{
  "from": "desktop/frontend/src/components/Composer.tsx",
  "symbol": "handleSubmit",
  "max_paths": 3,
  "include_go_internal": false
}
```

**Output:** summary line + JSON paths array.

#### `callgraph_trace_backward`

```json
{
  "go_method": "Submit",
  "receiver": "App",
  "include_events": true,
  "max_paths": 3
}
```

**Output:** UI 终点列表 + 完整路径。

#### `callgraph_find_bridge`

```json
{
  "frontend": { "file": ".../Composer.tsx", "line": 891 },
  "go_method": "App.Submit"
}
```

**Output:** 中间 bridge 节点 + 最短路径；找不到时返回候选 method 名（Levenshtein）。

### 7.3 待确认

- [ ] 是否需要第四个 tool `callgraph_affected_ui`（扁平 list）还是 merge 进 backward？
- [ ] tool 命名前缀 `callgraph_` vs `wails_`？
- [ ] 是否暴露 `callgraph_status`（与 dependency_status 对称）？

---

## 8. 与现有系统的集成

### 8.1 Dependency Graph 接口（callgraph 定义）

```go
type ModuleCatalog interface {
    ResolveFile(relPath string) (moduleID string, ok bool)
    ModuleKind(moduleID string) (string, error)
    EnsureReady(ctx context.Context) error
}
```

**dependency 侧：** 新增 `internal/dependency/catalog_adapter.go` 实现上述接口（Phase 1.5 小 PR，callgraph 设计确认后做）。

**Dependency Graph Phase 2 Bridge 字段：** callgraph 建好后，dependency 的 `CrossRealm` / `KindBridge` 可消费 callgraph 导出的 **method-level bridge edges 摘要**（单向依赖：dependency ← callgraph export，避免循环）。

### 8.2 CodeGraph 接口

```go
type SymbolQuery interface {
    Available() bool
    Callees(qualifiedSymbol string, depth int) ([]SymbolRef, error)
    Callers(qualifiedSymbol string, depth int) ([]SymbolRef, error)
}
```

- boot 若 MCP codegraph 已连接，注入 `mcpSymbolQuery`
- trace 工具 `include_go_internal=true` 时 **运行时查询** CodeGraph，结果 **不写入** index.json（避免 stale + 体积）；可选 LRU cache（TTL 60s，max 100 entries）

### 8.3 Verification Loop 集成

| 场景 | 注入 |
|------|------|
| 修改 `desktop/*.go` bind 方法 | verify retry + `BuildCrossRealmContext` |
| 修改返回 struct（models.ts / Go struct） | `AffectedUI` 摘要 |
| 修改 TS 组件 handler | forward trace 到 Go |
| 纯 Go internal/agent 变更 | 不注入（dependency 已有） |

**触发条件：** 与 dependency 相同 — `IsVerifyCommand` + 跨 realm 文件变更（catalog 判定 go+js 同时涉及）。

### 8.4 Boot Wiring

```go
// boot.go（伪代码）
if cfg.Callgraph.ShouldIndex(cfg.Dependency.ShouldIndex(...)) {
    depIdx.EnsureReady(ctx)
    cat := callgraph.NewDependencyCatalog(depIdx)
    cgIdx, _ := callgraph.Open(root)
    cgIdx.SetCatalog(cat)
    cgIdx.SetSymbolQuery(symbolQueryFromMCP(mcpRegistry))
    cgIdx.EnsureReady(ctx)
    callgraph.RegisterTools(reg, cgIdx)
    agent.SetCallgraphIndex(cgIdx)
    go cgIdx.RefreshIfStale(bgCtx)
}
```

**Config（arcdesk.toml）：**

```toml
[callgraph]
enabled = true          # 默认 false（首次运行）
auto_discover = true    # 探测 Wails 项目
```

### 8.5 待确认

- [ ] callgraph 默认 enabled 是否与 dependency 同步？
- [ ] CodeGraph 不可用时是否仍启用 callgraph（仅 RPC 链）？推荐 **是**
- [ ] `BuildCrossRealmContext` 与 `BuildFailureContext` 合并还是分开注入？

---

## 9. 边界情况与容错

| 场景 | 策略 |
|------|------|
| 动态 import bridge | 标记 `confidence: low`；find_bridge 提示 manual |
| wailsjs 不存在 | AppBindings + Go scan；stats.warnings |
| 多个 Go struct bind | NodeID 含 receiver；methodMap key=`Receiver.Method` |
| 方法名冲突 | 全名 `App.Submit`；短名 `Submit` 仅在唯一时解析 |
| 前端调用未 bind 方法 | bridge_call 节点存在，go_bind 缺失 → orphan_bridge edge + warning |
| Go bind 无前端调用 | gobind 节点标记 `unused_bind`（stats） |
| Events 动态 channel | 跳过；warning |
| `pnpm dev` mock 路径 | 排除 `makeMockApp` 函数体（bridge.ts 内标记 region） |
| 语法错误 TS/Go | 跳过文件，parseErrors[]，不 fail 整图 |
| CodeGraph 超时 | trace 返回 bind 层 + `"go_internal: unavailable"` |

### 9.1 待确认

- [ ] orphan bridge / unused bind 是否进 agent tool 输出还是仅 status？
- [ ] 是否支持 `BrowserOpenURL` 等 runtime 方法（非 go bind）作为节点？

---

## 10. 缓存与性能策略

### 10.1 持久化

```
.arcdesk/callgraph/
  index.json    # CallGraph snapshot (version 1)
  meta.json     # fingerprint, gitHead, indexVersion
```

**index.json 结构（与 dependency 对齐）：**

```json
{
  "version": 1,
  "root": "...",
  "builtAt": "...",
  "nodes": { "<NodeID>": { "kind", "name", "file", "line", "meta" } },
  "out": { "<NodeID>": ["<NodeID>", ...] },
  "in": { "<NodeID>": ["<NodeID>", ...] },
  "edges": [{ "from", "to", "kind", "meta" }],
  "bridgeByMethod": { "Submit": ["bridge:...", ...] },
  "methodMap": { "Submit": "gobind:...", "App.Submit": "gobind:..." },
  "eventByChannel": { "agent:event": { "emit": [...], "listen": [...] } },
  "stats": { "nodeCount", "bridgeCallCount", "goBindCount", "parseErrorCount", "warnings" }
}
```

**预估体积（ArcDesk）：** ~400–800 KB（小于 dependency index；无 files[] 重复）。

### 10.2 Staleness

`CheckStale` 触发条件（与 dependency 类似 + callgraph 特有）：

- `indexVersion` 变更
- `gitHead` 变更
- fingerprint 变更：`desktop/**/*.go`, `desktop/frontend/src/**/*.{ts,tsx}`, `wailsjs/go/main/App.d.ts`, `go.mod` mtime

**合并 fingerprint：** 可选读取 dependency meta fingerprint 作为输入之一（减少重复 walk）。

### 10.3 预计算 vs 按需

| 预计算（index build） | 按需（query time） |
|----------------------|-------------------|
| bridge 映射 | CodeGraph callees depth>1 |
| out/in 邻接表 | 路径格式化 |
| event channel 索引 | Levenshtein 候选 |
| go_bind AST depth-1 | — |

### 10.4 缓存命中设计

| 层级 | 策略 |
|------|------|
| 磁盘 | index.json 一次加载，Open 后内存驻留 |
| Index 读 | RLock，无拷贝 |
| CodeGraph | 进程内 LRU，key=`symbol:depth` |
| Refresh | TryLock 合并；与 dependency 并行 rebuild 共享 fingerprint walk |

**目标：**

- 冷启动 Open+Load：< 100ms
- EnsureReady（hit disk）：< 150ms
- TraceForward/Backward（hot）：< 5ms
- Full rebuild：< 3s

### 10.5 待确认

- [ ] index.json 是否存 `edges[]` 全量还是仅 out/in（dependency 已验证 edges 膨胀）？推荐 **out/in + bridgeByMethod**，edges 可选 omit
- [ ] callgraph 与 dependency 共用一个 meta.json 还是分开？推荐 **分开**（独立 stale）
- [ ] Refresh 是否与 dependency 合并为 single `.arcdesk/rebuild` 任务？

---

## 附录 A：ArcDesk 实测锚点（设计参考）

| 类别 | 真实样例 |
|------|----------|
| Go bind | `func (a *App) Submit(...)`, `ListTabs`, `StartTerminal` |
| 前端调用 | `import { app } from '../lib/bridge'` → `app.Submit()` |
| wailsjs | `desktop/frontend/wailsjs/go/main/App.d.ts` (~130 exports) |
| 事件 | Go: `runtime.EventsEmit(ctx, "agent:event", ...)` / FE: `onEvent()` → `EventsOn("agent:event")` |
| 二级包装 | `terminalBridge.ts` 订阅 `terminal:output` |

## 附录 B：Phase 分 delivery 建议

| Phase | 范围 |
|-------|------|
| **1a** | gobind scan + app.Method scan + bridge_invoke + trace_forward/backward + tools + persistence |
| **1b** | Events emit/listen + find_bridge + BuildCrossRealmContext |
| **1c** | CodeGraph MCP 按需扩展 + AffectedUI + dependency CrossRealm 回填 |

---

## 全局待确认清单（拍板项汇总）

1. **Scope：** Phase 1 是否包含 event 反向链，还是仅 RPC bridge？
2. **Naming：** 包名 `callgraph`，tool 前缀 `callgraph_`，缓存目录 `.arcdesk/callgraph/` — 是否 OK？
3. **ArcDesk 特化：** 是否假设唯一 bind receiver `*App`，还是泛化 Wails？
4. **TS 解析深度：** Phase 1 regex/heuristic vs 引入轻量 TS parser？
5. **CodeGraph：** 索引时 AST depth-1 vs 完全按需 MCP？
6. **Config 默认：** callgraph 首次运行 disabled（对齐 dependency/codegraph）？
7. **Dependency 耦合：** 先 callgraph 后 dependency adapter，还是同步交付 `ModuleCatalog`？
8. **index 体积：** 不存 edge files[]，与 dependency Phase 1 教训一致？

---

*Document version: 0.1 — implemented; see `internal/callgraph/` and agent verify retry wiring.*
