package callgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds callgraph tools to reg.
func RegisterTools(reg *tool.Registry, idx *Index) {
	if reg == nil || idx == nil {
		return
	}
	reg.Add(cgTraceForwardTool{idx: idx})
	reg.Add(cgTraceBackwardTool{idx: idx})
	reg.Add(cgFindBridgeTool{idx: idx})
	reg.Add(cgBreakpointsTool{idx: idx})
	reg.Add(cgStatusTool{idx: idx})
}

type cgStatusTool struct{ idx *Index }

func (cgStatusTool) Name() string { return "callgraph_status" }
func (cgStatusTool) Description() string {
	return "Wails cross-realm call graph status (UI↔Go bridge chains). Complements dependency_* (module level) and codegraph_* (Go symbols)."
}
func (cgStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (cgStatusTool) ReadOnly() bool { return true }
func (t cgStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	stats, err := t.idx.Status()
	if msg, err2 := toolResultOrNotReady("", err); err2 != nil {
		return "", err2
	} else if msg != "" {
		return msg, nil
	}
	b, _ := json.Marshal(stats)
	return fmt.Sprintf("Callgraph index: %d nodes, %d edges, %d bridge calls, %d go binds, %d event emits, %d event listens, %d event delivers",
		stats.NodeCount, stats.EdgeCount, stats.BridgeCallCount, stats.GoBindCount,
		stats.EventEmitCount, stats.EventListenCount, stats.EventDeliverCount) + "\n" + string(b), nil
}

type cgTraceForwardTool struct{ idx *Index }

func (cgTraceForwardTool) Name() string { return "callgraph_trace_forward" }
func (cgTraceForwardTool) Description() string {
	return "Trace forward from a React/TS component or hook to Go bind methods across the Wails bridge."
}
func (cgTraceForwardTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"from":{"type":"string","description":"file path or file#symbol"},"symbol":{"type":"string"},"max_paths":{"type":"integer"},"include_go_internal":{"type":"boolean"}},"required":["from"]}`)
}
func (cgTraceForwardTool) ReadOnly() bool { return true }
func (t cgTraceForwardTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		From              string `json:"from"`
		Symbol            string `json:"symbol"`
		MaxPaths          int    `json:"max_paths"`
		IncludeGoInternal bool   `json:"include_go_internal"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &p)
	}
	path, symbol := splitFromSymbol(p.From, p.Symbol)
	opts := DefaultTraceOptions()
	if p.MaxPaths > 0 {
		opts.MaxPaths = p.MaxPaths
	}
	opts.IncludeGoInternal = p.IncludeGoInternal
	paths, err := t.idx.TraceForward(ctx, path, symbol, opts)
	if msg, err2 := toolResultOrNotReady("", err); err2 != nil {
		return "", err2
	} else if msg != "" {
		return msg, nil
	}
	return FormatPathsSummary("Trace forward", paths) + "\n" + FormatPathsJSON(paths), nil
}

type cgTraceBackwardTool struct{ idx *Index }

func (cgTraceBackwardTool) Name() string { return "callgraph_trace_backward" }
func (cgTraceBackwardTool) Description() string {
	return "Trace backward from a Go App bind method to React/TS UI callers."
}
func (cgTraceBackwardTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"go_method":{"type":"string"},"receiver":{"type":"string"},"max_paths":{"type":"integer"},"include_events":{"type":"boolean"}},"required":["go_method"]}`)
}
func (cgTraceBackwardTool) ReadOnly() bool { return true }
func (t cgTraceBackwardTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		GoMethod      string `json:"go_method"`
		MaxPaths      int    `json:"max_paths"`
		IncludeEvents *bool  `json:"include_events"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &p)
	}
	method := strings.TrimSpace(p.GoMethod)
	if method == "" {
		return "", fmt.Errorf("go_method is required")
	}
	opts := DefaultTraceOptions()
	if p.MaxPaths > 0 {
		opts.MaxPaths = p.MaxPaths
	}
	if p.IncludeEvents != nil {
		opts.IncludeEvents = *p.IncludeEvents
	}
	paths, err := t.idx.TraceBackward(ctx, method, opts)
	if msg, err2 := toolResultOrNotReady("", err); err2 != nil {
		return "", err2
	} else if msg != "" {
		return msg, nil
	}
	return FormatPathsSummary("Trace backward", paths) + "\n" + FormatPathsJSON(paths), nil
}

type cgFindBridgeTool struct{ idx *Index }

func (cgFindBridgeTool) Name() string { return "callgraph_find_bridge" }
func (cgFindBridgeTool) Description() string {
	return "Find the shortest Wails bridge path between a frontend file/line and a Go bind method."
}
func (cgFindBridgeTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"frontend":{"type":"string","description":"file:line or file#symbol"},"go_method":{"type":"string"}},"required":["frontend","go_method"]}`)
}
func (cgFindBridgeTool) ReadOnly() bool { return true }
func (t cgFindBridgeTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Frontend string `json:"frontend"`
		GoMethod string `json:"go_method"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	path, line, symbol := parseFrontendRef(p.Frontend)
	if symbol != "" {
		paths, err := t.idx.TraceForward(ctx, path, symbol, DefaultTraceOptions())
		if msg, err2 := toolResultOrNotReady("", err); err2 != nil {
			return "", err2
		} else if msg != "" {
			return msg, nil
		}
		return FormatPathsSummary("Find bridge", paths) + "\n" + FormatPathsJSON(paths), nil
	}
	paths, err := t.idx.FindBridge(ctx, path, line, strings.TrimSpace(p.GoMethod))
	if msg, err2 := toolResultOrNotReady("", err); err2 != nil {
		return "", err2
	} else if msg != "" {
		return msg, nil
	}
	return FormatPathsSummary("Find bridge", paths) + "\n" + FormatPathsJSON(paths), nil
}

type cgBreakpointsTool struct{ idx *Index }

func (cgBreakpointsTool) Name() string { return "callgraph_breakpoints" }
func (cgBreakpointsTool) Description() string {
	return "Suggest debug breakpoints along the Wails UI→bridge→Go path for a frontend symbol and/or Go bind method."
}
func (cgBreakpointsTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"from":{"type":"string","description":"file path or file#symbol"},"symbol":{"type":"string"},"go_method":{"type":"string"},"changed_paths":{"type":"array","items":{"type":"string"}}}}`)
}
func (cgBreakpointsTool) ReadOnly() bool { return true }
func (t cgBreakpointsTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		From         string   `json:"from"`
		Symbol         string   `json:"symbol"`
		GoMethod       string   `json:"go_method"`
		ChangedPaths   []string `json:"changed_paths"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &p)
	}
	if len(p.ChangedPaths) > 0 {
		bps, err := t.idx.AutoBreakpoints(ctx, p.ChangedPaths, strings.TrimSpace(p.GoMethod))
		if msg, err2 := toolResultOrNotReady("", err); err2 != nil {
			return "", err2
		} else if msg != "" {
			return msg, nil
		}
		if len(bps) == 0 {
			return "No cross-realm breakpoints suggested (single-realm change or index empty).", nil
		}
		b, _ := json.Marshal(bps)
		return FormatBreakpointContext(bps) + "\n" + string(b), nil
	}
	path, symbol := splitFromSymbol(p.From, p.Symbol)
	bps, paths, err := t.idx.BreakpointsForQuery(ctx, path, symbol, strings.TrimSpace(p.GoMethod))
	if msg, err2 := toolResultOrNotReady("", err); err2 != nil {
		return "", err2
	} else if msg != "" {
		return msg, nil
	}
	b, _ := json.Marshal(bps)
	return FormatBreakpointContext(bps) + "\n" + FormatPathsSummary("Paths", paths) + "\n" + string(b), nil
}

func splitFromSymbol(from, symbol string) (path, sym string) {
	from = strings.TrimSpace(from)
	if i := strings.Index(from, "#"); i >= 0 {
		return from[:i], from[i+1:]
	}
	return from, strings.TrimSpace(symbol)
}

func parseFrontendRef(ref string) (path string, line int, symbol string) {
	ref = strings.TrimSpace(ref)
	if i := strings.Index(ref, "#"); i >= 0 {
		return ref[:i], 0, ref[i+1:]
	}
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		tail := ref[i+1:]
		if n, err := fmt.Sscanf(tail, "%d", &line); n == 1 && err == nil {
			return ref[:i], line, ""
		}
	}
	return ref, 0, ""
}
