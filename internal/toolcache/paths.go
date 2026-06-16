package toolcache

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"arcdesk/internal/toolstats"
)

// CachePaths returns workspace-relative paths referenced by a cacheable tool call.
func CachePaths(name, argsJSON string, ctx toolstats.KeyContext) []string {
	raw := strings.TrimSpace(argsJSON)
	if raw == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	switch name {
	case "read_file":
		if p, ok := m["path"].(string); ok {
			return []string{normalizeSlash(normalizePathForScope(ctx.WorkDir, p))}
		}
	case "grep", "ls":
		p, _ := m["path"].(string)
		if strings.TrimSpace(p) == "" {
			p = "."
		}
		return []string{normalizeSlash(normalizePathForScope(ctx.WorkDir, p))}
	case "glob":
		if pat, ok := m["pattern"].(string); ok {
			if dir := globScopeDir(pat); dir != "" {
				return []string{normalizeSlash(dir)}
			}
			return []string{"."}
		}
	}
	return nil
}

func normalizePathForScope(workDir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		p = "."
	}
	if workDir == "" {
		return filepath.ToSlash(filepath.Clean(p))
	}
	if filepath.IsAbs(p) {
		if rel, err := filepath.Rel(workDir, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(filepath.Clean(rel))
		}
	}
	if p == "." {
		return "."
	}
	return filepath.ToSlash(filepath.Clean(p))
}

func globScopeDir(pattern string) string {
	pattern = normalizeSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return "."
	}
	if before, _, ok := strings.Cut(pattern, "*"); ok {
		dir := strings.TrimRight(before, "/")
		if dir == "" {
			return "."
		}
		return dir
	}
	if strings.Contains(pattern, "/") {
		return filepath.Dir(pattern)
	}
	return "."
}

func normalizeSlash(p string) string {
	return strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
}

func pathsOverlap(cached, written string) bool {
	cached = normalizeSlash(cached)
	written = normalizeSlash(written)
	if cached == "" || written == "" {
		return false
	}
	if cached == written {
		return true
	}
	if cached == "." || written == "." {
		return true
	}
	if strings.HasPrefix(written, cached+"/") || strings.HasPrefix(cached, written+"/") {
		return true
	}
	return false
}

func entryAffectedByWrites(entryPaths, writtenPaths []string) bool {
	if len(entryPaths) == 0 {
		return true
	}
	for _, w := range writtenPaths {
		w = normalizeSlash(w)
		if w == "" {
			continue
		}
		for _, c := range entryPaths {
			if pathsOverlap(c, w) {
				return true
			}
		}
	}
	return false
}
