package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/config"
)

func TestFindWorkspaceLatestSession(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	older := writeScopedSession(t, dir, "older-project.jsonl", "project", projectRoot, "older", time.Now().Add(-2*time.Hour))
	newer := writeScopedSession(t, dir, "newer-project.jsonl", "project", projectRoot, "newer", time.Now().Add(-time.Hour))

	got := findWorkspaceLatestSession(dir, "project", projectRoot)
	if got != newer {
		t.Fatalf("latest = %q want %q (not %q)", got, newer, older)
	}
}

func TestFreshSessionSkipsWorkspaceLatest(t *testing.T) {
	tab := &WorkspaceTab{Scope: "project", WorkspaceRoot: filepath.Clean(t.TempDir()), freshSession: true}
	cands := tabSessionCandidates(tab, t.TempDir())
	for _, c := range cands {
		if strings.TrimSpace(c) != "" {
			t.Fatalf("fresh session should not reuse candidates, got %v", cands)
		}
	}
}
