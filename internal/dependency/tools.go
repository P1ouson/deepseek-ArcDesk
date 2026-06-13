package dependency

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds dependency graph tools to reg. idx may be nil (tools return
// a short unavailable message). Mirrors runtime reg.Add for MCP-style tools —
// codegraph uses MCP; dependency uses workspace-bound registry tools like
// slash_command.
func RegisterTools(reg *tool.Registry, idx *Index) {
	if reg == nil || idx == nil {
		return
	}
	reg.Add(depStatusTool{idx: idx})
	reg.Add(depAffectedByTool{idx: idx})
	reg.Add(depImportsTool{idx: idx})
	reg.Add(depCyclesTool{idx: idx})
}

type depStatusTool struct{ idx *Index }

func (depStatusTool) Name() string { return "dependency_status" }

func (depStatusTool) Description() string {
	return "Module/package-level dependency index status (not symbol-level — use codegraph_* for functions/callers). Returns node counts, build method, staleness, and parse error summary."
}

func (depStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}

func (depStatusTool) ReadOnly() bool { return true }

func (t depStatusTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	_ = args
	if t.idx == nil {
		return "dependency index unavailable", nil
	}
	stats, err := t.idx.Status()
	if err != nil {
		return "", err
	}
	summary := fmt.Sprintf("Dependency index: %d nodes, %d edges (go=%d js=%d), orphans=%d, build=%s, stale=%v, parse_errors=%d",
		stats.NodeCount, stats.EdgeCount, stats.GoPackages, stats.JSPackages, stats.OrphanCount, stats.BuildMethod, stats.Stale, stats.ParseErrorCount)
	b, _ := json.Marshal(stats)
	return summary + "\n" + string(b), nil
}

type depAffectedByTool struct{ idx *Index }

func (depAffectedByTool) Name() string { return "dependency_affected_by" }

func (depAffectedByTool) Description() string {
	return "Show module/package impact for a path or import (who imports this package). Module-level only — not codegraph_impact symbol analysis."
}

func (depAffectedByTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Repo-relative file/dir path or import/package name"}},"required":["path"]}`)
}

func (depAffectedByTool) ReadOnly() bool { return true }

func (t depAffectedByTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if strings.TrimSpace(p.Path) == "" {
		return "", fmt.Errorf("path is required")
	}
	res, err := t.idx.AffectedByPath(p.Path)
	if err != nil {
		return "", err
	}
	res = truncateImpactResult(res)
	summary := formatAffectedBySummary(res)
	b, _ := json.Marshal(res)
	return summary + "\n" + string(b), nil
}

type depImportsTool struct{ idx *Index }

func (depImportsTool) Name() string { return "dependency_imports" }

func (depImportsTool) Description() string {
	return "List direct module/package imports or importers for a path. Module-level adjacency — not symbol references."
}

func (depImportsTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"direction":{"type":"string","enum":["imports","imported_by"],"description":"imports = dependencies; imported_by = reverse importers"}},"required":["path"]}`)
}

func (depImportsTool) ReadOnly() bool { return true }

func (t depImportsTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Path      string `json:"path"`
		Direction string `json:"direction"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if strings.TrimSpace(p.Path) == "" {
		return "", fmt.Errorf("path is required")
	}
	id, err := t.idx.ResolveID(p.Path)
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(p.Direction)
	if dir == "" {
		dir = "imports"
	}
	var neighbors []NodeID
	switch dir {
	case "imports":
		neighbors, err = t.idx.ImportsOf(id)
	case "imported_by":
		neighbors, err = t.idx.ImportedBy(id)
	default:
		return "", fmt.Errorf("unknown direction %q", p.Direction)
	}
	if err != nil {
		return "", err
	}
	type row struct {
		ID   NodeID `json:"id"`
		Name string `json:"name,omitempty"`
		Kind Kind   `json:"kind,omitempty"`
	}
	rows := make([]row, 0, len(neighbors))
	for _, n := range neighbors {
		rows = append(rows, row{ID: n, Name: t.idx.NodeName(n), Kind: t.idx.NodeKind(n)})
	}
	b, _ := json.Marshal(map[string]any{"source": id, "direction": dir, "neighbors": rows})
	return fmt.Sprintf("%s (%s): %d neighbor(s)", id, dir, len(neighbors)) + "\n" + string(b), nil
}

type depCyclesTool struct{ idx *Index }

func (depCyclesTool) Name() string { return "dependency_cycles" }

func (depCyclesTool) Description() string {
	return "List source-import dependency cycles (Go/JS module level). Returns severity only — no fix hints in Phase 1."
}

func (depCyclesTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"lang":{"type":"string","enum":["go","js","all"],"description":"Filter cycles by language realm"}}}`)
}

func (depCyclesTool) ReadOnly() bool { return true }

func (t depCyclesTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Lang string `json:"lang"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &p)
	}
	cycles, err := t.idx.FindCycles()
	if err != nil {
		return "", err
	}
	filtered := filterCycles(cycles, p.Lang)
	summary := fmt.Sprintf("Dependency cycles: %d", len(filtered))
	if len(filtered) == 0 {
		return summary + "\n[]", nil
	}
	b, _ := json.Marshal(filtered)
	return summary + "\n" + string(b), nil
}

func truncateImpactResult(res ImpactResult) ImpactResult {
	const maxDirect, maxTransitive = 20, 30
	if len(res.Layers.Direct) > maxDirect {
		res.Layers.Direct = res.Layers.Direct[:maxDirect]
	}
	if len(res.Layers.Transitive) > maxTransitive {
		res.Layers.Transitive = res.Layers.Transitive[:maxTransitive]
	}
	return res
}

func formatAffectedBySummary(res ImpactResult) string {
	return fmt.Sprintf("Impact for %s: %d direct, %d transitive, %d external importer(s)/dep(s)",
		res.Source, len(res.Layers.Direct), len(res.Layers.Transitive), len(res.Layers.External))
}

func filterCycles(cycles []Cycle, lang string) []Cycle {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if lang == "" || lang == "all" {
		return cycles
	}
	var out []Cycle
	for _, c := range cycles {
		if c.Lang == lang {
			out = append(out, c)
		}
	}
	return out
}
