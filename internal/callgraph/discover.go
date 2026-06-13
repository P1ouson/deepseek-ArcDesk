package callgraph

import (
	"os"
	"path/filepath"
	"strings"
)

// Discoverable reports whether root looks like a Wails desktop project.
func Discoverable(root string) bool {
	root = strings.TrimSpace(root)
	if root == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, "desktop", "main.go")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, "desktop", "frontend", "wailsjs")); err != nil {
		return false
	}
	return true
}
