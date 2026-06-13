package gitrag

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Discoverable reports whether workspace looks like a git checkout.
func Discoverable(root string) bool {
	root = strings.TrimSpace(root)
	if root == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	_, err := gitTopLevel(ctx, root)
	return err == nil
}
