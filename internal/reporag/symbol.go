package reporag

import (
	"context"
	"fmt"
	"strings"

	"arcdesk/internal/callgraph"
)

// SearchSymbol finds symbols via CodeGraph search, then Wails callgraph traces.
func (h *Host) SearchSymbol(ctx context.Context, query, file string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	if h == nil {
		return "", fmt.Errorf("repo host unavailable")
	}

	if h.Reg != nil && h.CodegraphEnabled {
		if out, err := codegraphCall(h.Reg, ctx, "codegraph_search", map[string]any{
			"query": query,
			"limit": 10,
		}); err == nil && strings.TrimSpace(out) != "" {
			return "## Symbol Graph (CodeGraph)\n" + strings.TrimSpace(out), nil
		}
	}

	if h.Callgraph != nil && file != "" {
		path, symbol := splitFileSymbol(file, query)
		if symbol == "" {
			symbol = query
		}
		paths, err := h.Callgraph.TraceForward(ctx, path, symbol, callgraph.DefaultTraceOptions())
		if err == nil && len(paths) > 0 {
			return "## Symbol Graph (Wails callgraph)\n" + callgraph.FormatPathsSummary("Forward trace", paths) + "\n" + callgraph.FormatPathsJSON(paths), nil
		}
	}

	if h.Dep != nil {
		if id, err := h.Dep.ResolveID(query); err == nil && id != "" {
			if st, err := h.Dep.Status(); err == nil {
				return fmt.Sprintf("Dependency module %q resolved (%d modules indexed). Use dependency_imports or dependency_affected_by for graph queries.", id, st.NodeCount), nil
			}
		}
	}

	return formatUnavailable("symbol graph") + " Searched query: " + query, nil
}

func splitFileSymbol(file, fallback string) (path, symbol string) {
	file = strings.TrimSpace(file)
	if i := strings.Index(file, "#"); i >= 0 {
		return file[:i], file[i+1:]
	}
	if file != "" {
		return file, strings.TrimSpace(fallback)
	}
	return "", strings.TrimSpace(fallback)
}
