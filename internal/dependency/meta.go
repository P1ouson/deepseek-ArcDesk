package dependency

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

var fingerprintFiles = map[string]bool{
	"go.mod": true, "go.sum": true,
	"package.json": true, "pnpm-lock.yaml": true,
	"package-lock.json": true, "yarn.lock": true,
	"bun.lockb": true, "bun.lock": true,
}

// ComputeFingerprint hashes mtimes of go.mod, lockfiles, and all package.json files.
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
	_ = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if goWalkSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !fingerprintFiles[name] && name != "package.json" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			rel = path
		}
		parts = append(parts, fmtFingerprintPart(normalizeSlash(rel), info.ModTime()))
		return nil
	})
	if len(parts) == 0 {
		return ""
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:8])
}

func fmtFingerprintPart(rel string, mod time.Time) string {
	return rel + ":" + mod.UTC().Format(time.RFC3339Nano)
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
