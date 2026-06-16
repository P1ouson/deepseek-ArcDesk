package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const gitignoreArcDeskMarker = "# ArcDesk local workspace"

var gitignoreArcDeskBlock = strings.Join([]string{
	"",
	gitignoreArcDeskMarker + " (indexes, session metadata — not project source)",
	".arcdesk/",
}, "\n")

// InitProjectGitRepository initializes git in workspaceRoot and ensures .arcdesk/
// is listed in .gitignore so ArcDesk caches are not shown as first commits.
func (a *App) InitProjectGitRepository(workspaceRoot string) ShellRunResult {
	root := normalizeProjectRoot(workspaceRoot)
	if root == "" {
		return ShellRunResult{Err: "workspace path is required"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	out, err := runDetachedShell(ctx, "git init", root)
	if err != nil {
		msg := err.Error()
		if strings.TrimSpace(out) != "" {
			msg = strings.TrimSpace(out)
		}
		return ShellRunResult{Output: out, Err: msg}
	}
	if err := ensureProjectGitignore(root); err != nil {
		return ShellRunResult{Output: out, Err: err.Error()}
	}
	return ShellRunResult{Output: strings.TrimSpace(out)}
}

func ensureProjectGitignore(root string) error {
	path := filepath.Join(root, ".gitignore")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		body := strings.TrimPrefix(gitignoreArcDeskBlock, "\n") + "\n"
		return os.WriteFile(path, []byte(body), 0o644)
	}
	if gitignoreCoversArcDesk(string(existing)) {
		return nil
	}
	body := strings.TrimRight(string(existing), "\n") + gitignoreArcDeskBlock + "\n"
	return os.WriteFile(path, []byte(body), 0o644)
}

func gitignoreCoversArcDesk(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		field := strings.TrimSpace(line)
		if field == "" || strings.HasPrefix(field, "#") {
			continue
		}
		field = strings.TrimSuffix(field, "/")
		if field == ".arcdesk" || strings.HasPrefix(field, ".arcdesk/") || strings.Contains(field, ".arcdesk") {
			return true
		}
	}
	return false
}
