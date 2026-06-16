package toolstats

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
)

// KeyContext supplies workspace context for Phase-2 intent normalization.
type KeyContext struct {
	WorkDir   string
	Normalize bool
}

// IntentKey builds a cache/reuse identity after tool-specific arg normalization.
func IntentKey(name, argsJSON string, ctx KeyContext) string {
	if ctx.Normalize {
		argsJSON = NormalizeToolArgs(name, argsJSON, ctx.WorkDir)
	}
	return Key(name, argsJSON)
}

// NormalizeToolArgs applies tool-specific canonicalization so equivalent calls
// share a cache key (path aliases, default arg omission, pattern slashes).
func NormalizeToolArgs(name, argsJSON, workDir string) string {
	raw := strings.TrimSpace(argsJSON)
	if raw == "" {
		return raw
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	m, ok := v.(map[string]any)
	if !ok {
		return string(canonicalJSON(v))
	}
	switch name {
	case "read_file":
		normalizeReadFileArgs(m, workDir)
	case "grep":
		normalizeGrepArgs(m, workDir)
	case "glob":
		normalizeGlobArgs(m)
	case "ls":
		normalizeLsArgs(m, workDir)
	}
	return string(canonicalJSON(m))
}

func normalizeReadFileArgs(v map[string]any, workDir string) {
	if p, ok := v["path"].(string); ok {
		v["path"] = normalizePathKey(workDir, p)
	}
	if off, ok := asInt(v["offset"]); ok && off == 0 {
		delete(v, "offset")
	}
	if lim, ok := asInt(v["limit"]); ok && lim <= 0 {
		delete(v, "limit")
	}
}

func normalizeGrepArgs(v map[string]any, workDir string) {
	p, _ := v["path"].(string)
	if strings.TrimSpace(p) == "" {
		p = "."
	}
	v["path"] = normalizePathKey(workDir, p)
	if pat, ok := v["pattern"].(string); ok {
		v["pattern"] = strings.TrimSpace(pat)
	}
}

func normalizeGlobArgs(v map[string]any) {
	if pat, ok := v["pattern"].(string); ok {
		v["pattern"] = normalizeGlobPattern(pat)
	}
}

func normalizeLsArgs(v map[string]any, workDir string) {
	p, _ := v["path"].(string)
	if strings.TrimSpace(p) == "" {
		p = "."
	}
	v["path"] = normalizePathKey(workDir, p)
	if rec, ok := v["recursive"].(bool); ok && !rec {
		delete(v, "recursive")
	}
}

func normalizePathKey(workDir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		p = "."
	}
	resolved := resolvePath(workDir, p)
	resolved = filepath.Clean(resolved)
	if runtime.GOOS == "windows" {
		resolved = strings.ToLower(resolved)
	}
	return filepath.ToSlash(resolved)
}

func resolvePath(workDir, p string) string {
	if workDir == "" {
		if filepath.IsAbs(p) {
			return p
		}
		return p
	}
	if p == "" || p == "." {
		return workDir
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(workDir, p)
}

func normalizeGlobPattern(p string) string {
	p = strings.TrimSpace(p)
	return filepath.ToSlash(p)
}

func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		return int(x), true
	case int:
		return x, true
	case int64:
		return int(x), true
	default:
		return 0, false
	}
}
