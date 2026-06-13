package callgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"arcdesk/internal/proc"
)

var walkSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	".arcdesk": true, "dist": true, "build": true,
}

// ComputeFingerprint hashes mtimes of Wails-relevant source files.
func ComputeFingerprint(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	var parts []string

	add := func(rel string, mod time.Time) {
		rel = normalizeSlash(rel)
		if rel != "" {
			parts = append(parts, rel+":"+mod.UTC().Format(time.RFC3339Nano))
		}
	}

	for _, rel := range []string{"go.mod", "go.sum"} {
		info, err := os.Stat(filepath.Join(abs, rel))
		if err == nil && !info.IsDir() {
			add(rel, info.ModTime())
		}
	}

	_ = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if walkSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return nil
		}
		rel = normalizeSlash(rel)
		if !fingerprintRelevant(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		add(rel, info.ModTime())
		return nil
	})

	if len(parts) == 0 {
		return ""
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:8])
}

func fingerprintRelevant(rel string) bool {
	if rel == "desktop/frontend/wailsjs/go/main/App.d.ts" {
		return true
	}
	if strings.HasPrefix(rel, "desktop/") && strings.HasSuffix(rel, ".go") {
		return true
	}
	if strings.HasPrefix(rel, "desktop/frontend/src/") {
		switch {
		case strings.HasSuffix(rel, ".ts"), strings.HasSuffix(rel, ".tsx"):
			return true
		}
	}
	return false
}

// CheckStale reports whether meta is out of date for root.
func CheckStale(root string, meta *Meta) bool {
	if meta == nil {
		return true
	}
	if meta.IndexVersion != IndexVersion {
		return true
	}
	head := gitHead(root)
	if head != "" && meta.GitHead != "" && head != meta.GitHead {
		return true
	}
	return ComputeFingerprint(root) != meta.Fingerprint
}

func gitHead(root string) string {
	cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	proc.HideWindowDetached(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// NewMeta builds meta for a fresh index.
func NewMeta(root string) *Meta {
	return &Meta{
		GeneratedAt:  time.Now().UTC(),
		GitHead:      gitHead(root),
		Fingerprint:  ComputeFingerprint(root),
		IndexVersion: IndexVersion,
	}
}
