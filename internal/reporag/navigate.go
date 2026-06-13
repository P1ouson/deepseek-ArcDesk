package reporag

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"arcdesk/internal/callgraph"
)

// Navigate resolves definitions or references using LSP, CodeGraph, or callgraph.
func (h *Host) Navigate(ctx context.Context, mode, file string, line int, symbol string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "definition"
	}
	file = filepath.ToSlash(strings.TrimSpace(file))
	symbol = strings.TrimSpace(symbol)
	if file == "" || symbol == "" || line < 1 {
		return "", fmt.Errorf("file, line (>=1), and symbol are required")
	}
	if h == nil {
		return "", fmt.Errorf("repo host unavailable")
	}

	if h.LSP != nil {
		var (
			out string
			err error
		)
		switch mode {
		case "references", "refs", "reference":
			out, err = h.LSP.References(ctx, file, line, symbol)
		default:
			out, err = h.LSP.Definition(ctx, file, line, symbol)
		}
		if err == nil && strings.TrimSpace(out) != "" && !strings.Contains(strings.ToLower(out), "no language server") {
			return "## Code Navigation (LSP)\n" + strings.TrimSpace(out), nil
		}
	}

	if h.Reg != nil && h.CodegraphEnabled {
		toolName := "codegraph_node"
		if mode == "references" || mode == "refs" || mode == "reference" {
			toolName = "codegraph_callers"
		}
		payload := map[string]any{"symbol": symbol, "file": file, "line": line}
		if out, err := codegraphCall(h.Reg, ctx, toolName, payload); err == nil && strings.TrimSpace(out) != "" {
			return "## Code Navigation (CodeGraph)\n" + strings.TrimSpace(out), nil
		}
		if out, err := codegraphCall(h.Reg, ctx, "codegraph_search", map[string]any{"query": symbol, "limit": 8}); err == nil && strings.TrimSpace(out) != "" {
			return "## Code Navigation (CodeGraph search)\n" + strings.TrimSpace(out), nil
		}
	}

	if h.Callgraph != nil && isFrontendFile(file) {
		paths, err := h.Callgraph.TraceForward(ctx, file, symbol, callgraph.DefaultTraceOptions())
		if err == nil && len(paths) > 0 {
			return "## Code Navigation (Wails callgraph)\n" + callgraph.FormatPathsSummary("UI→Go", paths) + "\n" + callgraph.FormatPathsJSON(paths), nil
		}
	}

	return formatUnavailable("code navigation") + fmt.Sprintf(" (%s %s:%d %s)", mode, file, line, symbol), nil
}

func isFrontendFile(path string) bool {
	path = strings.ToLower(path)
	return strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") ||
		strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".jsx")
}
