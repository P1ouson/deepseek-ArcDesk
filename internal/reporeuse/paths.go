package reporeuse

import (
	"path/filepath"
	"strings"
)

// PathsAffectDependency reports whether any path can change the module graph.
func PathsAffectDependency(paths []string) bool {
	for _, p := range paths {
		if pathAffectsDependency(p) {
			return true
		}
	}
	return false
}

func pathAffectsDependency(rel string) bool {
	rel = normalizeSlash(rel)
	if rel == "" {
		return false
	}
	base := strings.ToLower(filepath.Base(rel))
	switch base {
	case "go.mod", "go.sum",
		"package.json", "pnpm-lock.yaml", "package-lock.json",
		"yarn.lock", "bun.lock", "bun.lockb":
		return true
	}
	ext := strings.ToLower(filepath.Ext(rel))
	switch ext {
	case ".go", ".mod", ".sum":
		return true
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return true
	}
	return false
}

// PathsAffectCallgraph reports whether any path can change the Wails callgraph index.
func PathsAffectCallgraph(paths []string) bool {
	for _, p := range paths {
		if pathAffectsCallgraph(p) {
			return true
		}
	}
	return false
}

func pathAffectsCallgraph(rel string) bool {
	rel = normalizeSlash(rel)
	if rel == "" {
		return false
	}
	if rel == "go.mod" || rel == "go.sum" {
		return true
	}
	if rel == "desktop/frontend/wailsjs/go/main/App.d.ts" {
		return true
	}
	if strings.HasPrefix(rel, "desktop/") && strings.HasSuffix(rel, ".go") {
		return true
	}
	if strings.HasPrefix(rel, "desktop/frontend/src/") {
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".ts", ".tsx", ".js", ".jsx":
			return true
		}
	}
	return false
}

func normalizeSlash(p string) string {
	return strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
}
