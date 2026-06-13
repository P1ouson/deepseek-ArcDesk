package callgraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"arcdesk/internal/tool"
)

// SymbolRef is a Go symbol returned by CodeGraph callees queries.
type SymbolRef struct {
	Name string
	File string
	Line int
	Kind string // function, method
}

// SymbolQuery resolves Go callees on demand (not persisted in the call graph).
type SymbolQuery interface {
	Available() bool
	Callees(ctx context.Context, symbol string, depth int) ([]SymbolRef, error)
}

// MCPToolCaller executes a named MCP tool with JSON args.
type MCPToolCaller interface {
	Available() bool
	CallTool(ctx context.Context, name string, args json.RawMessage) (string, error)
}

type mcpSymbolQuery struct {
	caller  MCPToolCaller
	timeout time.Duration
}

type noopSymbolQuery struct{}

func (noopSymbolQuery) Available() bool { return false }

func (noopSymbolQuery) Callees(context.Context, string, int) ([]SymbolRef, error) {
	return nil, errors.New("symbol query unavailable")
}

// NewSymbolQuery returns a SymbolQuery backed by an MCP tool caller.
func NewSymbolQuery(caller MCPToolCaller) SymbolQuery {
	if caller == nil || !caller.Available() {
		return noopSymbolQuery{}
	}
	return &mcpSymbolQuery{caller: caller, timeout: 5 * time.Second}
}

func (q *mcpSymbolQuery) Available() bool {
	return q != nil && q.caller != nil && q.caller.Available()
}

func (q *mcpSymbolQuery) Callees(ctx context.Context, symbol string, depth int) ([]SymbolRef, error) {
	if !q.Available() {
		return nil, errors.New("symbol query unavailable")
	}
	if depth <= 0 {
		depth = 3
	}
	reqCtx := ctx
	if q.timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, q.timeout)
		defer cancel()
	}
	args, _ := json.Marshal(map[string]any{
		"symbol": symbol,
		"depth":  depth,
	})
	raw, err := q.caller.CallTool(reqCtx, "codegraph_callees", args)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("symbol query timeout: %w", err)
		}
		return nil, err
	}
	return parseCalleesResponse(raw)
}

func parseCalleesResponse(raw string) ([]SymbolRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var payload struct {
		Callees []struct {
			Name string `json:"name"`
			File string `json:"file"`
			Line int    `json:"line"`
			Kind string `json:"kind"`
		} `json:"callees"`
		Results []struct {
			Name string `json:"name"`
			File string `json:"file"`
			Line int    `json:"line"`
			Kind string `json:"kind"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	src := payload.Callees
	if len(src) == 0 {
		src = payload.Results
	}
	out := make([]SymbolRef, 0, len(src))
	for _, c := range src {
		if c.Name == "" {
			continue
		}
		out = append(out, SymbolRef{
			Name: c.Name,
			File: c.File,
			Line: c.Line,
			Kind: c.Kind,
		})
	}
	return out, nil
}

type registryToolCaller struct {
	reg *tool.Registry
}

const codegraphCalleesTool = "mcp__codegraph__codegraph_callees"

// NewSymbolQueryFromRegistry builds a SymbolQuery backed by the tool registry.
func NewSymbolQueryFromRegistry(reg *tool.Registry) SymbolQuery {
	if reg == nil {
		return noopSymbolQuery{}
	}
	return NewSymbolQuery(registryToolCaller{reg: reg})
}

func (r registryToolCaller) Available() bool {
	if r.reg == nil {
		return false
	}
	_, ok := r.reg.Get(codegraphCalleesTool)
	return ok
}

func (r registryToolCaller) CallTool(ctx context.Context, _ string, args json.RawMessage) (string, error) {
	if r.reg == nil {
		return "", errors.New("registry unavailable")
	}
	t, ok := r.reg.Get(codegraphCalleesTool)
	if !ok {
		return "", errors.New("codegraph callees tool unavailable")
	}
	return t.Execute(ctx, args)
}

// MockSymbolQuery is a test double for SymbolQuery.
type MockSymbolQuery struct {
	OK      bool
	Results []SymbolRef
	Err     error
	Delay   time.Duration
}

func (m MockSymbolQuery) Available() bool { return m.OK }

func (m MockSymbolQuery) Callees(ctx context.Context, _ string, _ int) ([]SymbolRef, error) {
	if m.Delay > 0 {
		select {
		case <-time.After(m.Delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Results, nil
}
