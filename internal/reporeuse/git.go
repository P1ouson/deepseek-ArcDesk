// Package reporeuse provides git diff and path-classification helpers for
// Phase-4 partial index reuse (dependency/callgraph meta bump).
package reporeuse

import (
	"os/exec"
	"strings"

	"arcdesk/internal/proc"
)

// HeadChangedFingerprintStable reports git HEAD moved but fingerprint inputs unchanged.
func HeadChangedFingerprintStable(oldHead, newHead, oldFP, newFP string) bool {
	oldHead = strings.TrimSpace(oldHead)
	newHead = strings.TrimSpace(newHead)
	if oldHead == "" || newHead == "" || oldHead == newHead {
		return false
	}
	if oldFP == "" || newFP == "" {
		return false
	}
	return oldFP == newFP
}

// ChangedFilesBetween lists repo-relative paths changed between two commits.
func ChangedFilesBetween(root, oldHead, newHead string) ([]string, error) {
	root = strings.TrimSpace(root)
	oldHead = strings.TrimSpace(oldHead)
	newHead = strings.TrimSpace(newHead)
	if root == "" || oldHead == "" || newHead == "" || oldHead == newHead {
		return nil, nil
	}
	cmd := exec.Command("git", "-C", root, "diff", "--name-only", oldHead, newHead)
	proc.HideWindowDetached(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	lines := strings.Split(raw, "\n")
	outPaths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.ReplaceAll(line, "\\", "/"))
		if line != "" {
			outPaths = append(outPaths, line)
		}
	}
	return outPaths, nil
}
