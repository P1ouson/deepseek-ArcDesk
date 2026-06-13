package dependency

import (
	"os"
	"path/filepath"
	"strings"
)

const discoverMaxDepth = 6

var discoverSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	".arcdesk": true, "dist": true, "build": true,
}

// Discoverable reports whether root looks like a Go or JS/TS project worth indexing.
func Discoverable(root string) bool {
	root = strings.TrimSpace(root)
	if root == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		return true
	}
	return findPackageJSON(root, discoverMaxDepth)
}

func findPackageJSON(dir string, depth int) bool {
	if depth < 0 {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() || discoverSkipDirs[e.Name()] {
			continue
		}
		if findPackageJSON(filepath.Join(dir, e.Name()), depth-1) {
			return true
		}
	}
	return false
}
