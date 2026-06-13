package reporag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds repo_status, repo_symbol, and repo_navigate tools.
func RegisterTools(reg *tool.Registry, host *Host) {
	if reg == nil || host == nil {
		return
	}
	reg.Add(repoStatusTool{host: host})
	reg.Add(repoSymbolTool{host: host})
	reg.Add(repoNavigateTool{host: host})
}

type repoStatusTool struct{ host *Host }

func (repoStatusTool) Name() string { return "repo_status" }
func (repoStatusTool) Description() string {
	return "Unified repo-aware RAG status: dependency graph, symbol graph (CodeGraph/callgraph), code navigation (LSP), and repomap."
}
func (repoStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (repoStatusTool) ReadOnly() bool { return true }
func (t repoStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	report := t.host.Status(ctx)
	b, _ := json.Marshal(report)
	var lines []string
	for _, layer := range report.Layers {
		state := "ready"
		if !layer.Ready {
			state = "not ready"
		}
		if layer.Stale {
			state += ", stale"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s (%s)", layer.Name, state, layer.Detail))
	}
	return strings.Join(lines, "\n") + "\n" + string(b), nil
}

type repoSymbolTool struct{ host *Host }

func (repoSymbolTool) Name() string { return "repo_symbol" }
func (repoSymbolTool) Description() string {
	return "Search the repo symbol graph: CodeGraph for Go/TS symbols, Wails callgraph for UI→bridge chains, dependency module lookup fallback."
}
func (repoSymbolTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"file":{"type":"string","description":"optional file#symbol anchor"}},"required":["query"]}`)
}
func (repoSymbolTool) ReadOnly() bool { return true }
func (t repoSymbolTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Query string `json:"query"`
		File  string `json:"file"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &p)
	}
	if strings.TrimSpace(p.Query) == "" {
		return "", fmt.Errorf("query is required")
	}
	return t.host.SearchSymbol(ctx, p.Query, p.File)
}

type repoNavigateTool struct{ host *Host }

func (repoNavigateTool) Name() string { return "repo_navigate" }
func (repoNavigateTool) Description() string {
	return "Jump to definition or find references: LSP first, then CodeGraph, then Wails callgraph for frontend files."
}
func (repoNavigateTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"mode":{"type":"string","enum":["definition","references"]},"file":{"type":"string"},"line":{"type":"integer"},"symbol":{"type":"string"}},"required":["file","line","symbol"]}`)
}
func (repoNavigateTool) ReadOnly() bool { return true }
func (t repoNavigateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Mode   string `json:"mode"`
		File   string `json:"file"`
		Line   int    `json:"line"`
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	return t.host.Navigate(ctx, p.Mode, p.File, p.Line, p.Symbol)
}
