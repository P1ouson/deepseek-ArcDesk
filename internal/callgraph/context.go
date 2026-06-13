package callgraph

import (
	"context"
	"strings"
)

var verifyKeywords = []string{"build", "test", "vet", "lint", "compile"}

// IsVerifyCommand reports whether cmd looks like a verification command.
func IsVerifyCommand(cmd string) bool {
	cmd = strings.ToLower(strings.TrimSpace(cmd))
	if cmd == "" {
		return false
	}
	for _, kw := range verifyKeywords {
		if strings.Contains(cmd, kw) {
			return true
		}
	}
	return false
}

// BuildCrossRealmContext formats Wails call chain context for cross-realm verify retries.
func BuildCrossRealmContext(idx *Index, changedPaths []string, failedCmd string) string {
	if !IsVerifyCommand(failedCmd) || idx == nil {
		return ""
	}
	if !crossRealmChange(idx, changedPaths) {
		return ""
	}
	stats, err := idx.Status()
	if err != nil || stats.NodeCount == 0 {
		return ""
	}

	var blocks []string
	seen := map[string]bool{}
	for _, path := range changedPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		g, err := idx.graphForRead()
		if err != nil {
			continue
		}
		id, err := ResolveNodeID(g, path, "")
		if err != nil {
			for _, p := range pathsFromGoFile(g, path) {
				if seen[p.ID] {
					continue
				}
				seen[p.ID] = true
				if block := FormatLLMContext([]CallPath{p}, ""); block != "" {
					blocks = append(blocks, block)
				}
			}
			continue
		}
		paths := TraceForward(g, id, DefaultTraceOptions())
		if len(paths) == 0 {
			paths = TraceBackward(g, id, DefaultTraceOptions())
		}
		for _, p := range paths {
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			if block := FormatLLMContext([]CallPath{p}, ""); block != "" {
				blocks = append(blocks, block)
			}
		}
	}
	if len(blocks) == 0 {
		return ""
	}
	out := blocks[0]
	if len(blocks) > 1 {
		out = strings.Join(blocks[:min(2, len(blocks))], "\n")
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 8 {
		lines = lines[:8]
	}
	result := strings.Join(lines, "\n")

	if bps, err := idx.AutoBreakpoints(context.Background(), changedPaths, ""); err == nil && len(bps) > 0 {
		if bpBlock := FormatBreakpointContext(bps); bpBlock != "" {
			result = result + "\n\n" + bpBlock
		}
	}
	return result
}

func crossRealmChange(idx *Index, paths []string) bool {
	if idx == nil || idx.catalog == nil {
		return false
	}
	hasGo, hasJS := false, false
	for _, p := range paths {
		p = normalizeSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "desktop/") && strings.HasSuffix(p, ".go") {
			hasGo = true
		}
		if strings.HasPrefix(p, "desktop/frontend/src/") {
			hasJS = true
		}
		if mod, ok := idx.catalog.ResolveFile(p); ok {
			if k, ok := idx.catalog.ModuleKind(mod); ok {
				switch k {
				case "go", "gomod":
					hasGo = true
				case "js":
					hasJS = true
				}
			}
		}
	}
	return hasGo && hasJS
}

func pathsFromGoFile(g *CallGraph, path string) []CallPath {
	path = normalizeSlash(path)
	var out []CallPath
	for _, n := range g.Nodes {
		if n != nil && n.Kind == KindGoBind && n.File == path {
			out = append(out, TraceBackward(g, n.ID, DefaultTraceOptions())...)
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
