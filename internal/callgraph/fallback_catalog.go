package callgraph

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// NewFallbackCatalog returns a file-walking catalog when dependency index is nil.
func NewFallbackCatalog(root string) ModuleCatalog {
	return &fallbackCatalog{root: strings.TrimSpace(root)}
}

type fallbackCatalog struct {
	root string
}

func (f *fallbackCatalog) ResolveFile(relPath string) (string, bool) {
	relPath = normalizeSlash(relPath)
	if relPath == "" || f.root == "" {
		return "", false
	}
	abs := filepath.Join(f.root, filepath.FromSlash(relPath))
	if _, err := os.Stat(abs); err != nil {
		return "", false
	}
	switch {
	case strings.HasPrefix(relPath, "desktop/frontend/"):
		return "js:" + relPath, true
	case strings.HasSuffix(relPath, ".go"):
		return "go:" + filepath.Dir(relPath), true
	case relPath == "go.mod" || strings.HasSuffix(relPath, "/go.mod"):
		return "gomod:" + relPath, true
	default:
		return "file:" + relPath, true
	}
}

func (f *fallbackCatalog) ModuleKind(moduleID string) (string, bool) {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return "", false
	}
	if strings.HasPrefix(moduleID, "js:") {
		return "js", true
	}
	if strings.HasPrefix(moduleID, "gomod:") {
		return "gomod", true
	}
	if strings.HasPrefix(moduleID, "go:") {
		return "go", true
	}
	if strings.HasPrefix(moduleID, "bridge:") {
		return "bridge", true
	}
	return "file", true
}

func (f *fallbackCatalog) EnsureReady(context.Context) error { return nil }

func (f *fallbackCatalog) Status() (int, int, string) { return 0, 0, "fallback" }
