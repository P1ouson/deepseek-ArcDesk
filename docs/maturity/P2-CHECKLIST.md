# P2 — Differentiation Checklist

| # | Capability | Package / tools | Code | Coverage | Dogfood |
|---|------------|-----------------|------|----------|---------|
| 13 | Runtime RAG | `runtime` → `runtime_find` | ✅ | ≥90% | ✅ |
| 14 | UI-aware RAG | `uirag` → `ui_*` | ✅ | ≥90% | ✅ |
| 15 | Architecture Guardian | `guardian` → `architecture_guardian_*` | ✅ | ≥90% | ✅ |
| 16 | Task DAG | `taskdag` → `taskdag_*` | ✅ | ≥90% | ✅ |
| 17 | Cost router | `costrouter` → `cost_router_*` | ✅ | ≥90% | ✅ |
| 18 | Context compression | `ctxcompress` → `context_compression_status` | ✅ | ≥90% | ✅ |

Boot smoke: `go test ./internal/boot/... -run TestBuildRegistersP2ToolsFromRepoRoot`

Restart ArcDesk after Go changes so tools reload.

---

## Dogfood 脚本（在 ArcDesk 里逐条粘贴）

工作区建议：`p0-dogfood-demo`（`C:\Users\dell\Desktop\p0-dogfood-demo`）或本仓库 `DeepSeek-ARCDESK`（有 `desktop/frontend` 和 `SPEC.md`）。2026-06-13 dogfood 在 demo 项目全部通过。

### ① Runtime RAG (#13)

```
先调用 runtime_status。然后调用 runtime_find，query 填 "error" 或你刚看到的报错关键词，errors_only 设为 true。把匹配条数和最旧一条的内容简要告诉我。
```

### ② UI RAG (#14)

```
调用 ui_status，再 ui_list limit=20，ui_find query="Message" 或 "Chat"，对第一个命中调用 ui_read name_or_path=<组件名> limit=40。总结组件文件路径和导出名称。
```

### ③ Architecture Guardian (#15)

```
调用 architecture_guardian_rules，再 architecture_guardian_check path="internal/agent/agent.go" new_text="func (a *App) Demo() {}".说明是否 blocked 以及违反哪条 SPEC。
```

### ④ Task DAG (#16)

```
用 taskdag_load 加载 plan：
- db: Init schema
- auth: Add login (deps: db)
- ui: Wire form (deps: auth)
然后 taskdag_ready，对 ready 的任务依次 taskdag_start + taskdag_complete（带 summary），直到 taskdag_status 显示全部完成。
```

### ⑤ Cost router (#17)

```
调用 cost_router_status。再两次 cost_router_classify：一次 prompt="how does verification work?"，一次 prompt="explore the codebase for guardian tools"。对比 tier 和 model 字段。
```

### ⑥ Context compression (#18)

```
调用 context_compression_status，确认 enabled 和 tool_output_max_bytes（默认启用时为 16384）。无需改代码。
```

---

## 通过标准

- 6 条 dogfood 均能调用对应工具且无 “tool not found”
- Guardian check 对 Wails bind 越界路径应 **blocked**（或明确 warn）
- Task DAG 必须按依赖顺序完成，不能跳过 deps
- 全部 P2 包 `go test -cover` ≥ 90%
