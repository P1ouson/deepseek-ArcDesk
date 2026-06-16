package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitignoreCoversArcDesk(t *testing.T) {
	cases := map[string]bool{
		"":                        false,
		"node_modules/\n":         false,
		".arcdesk/\n":              true,
		".arcdesk\n":               true,
		"**/.arcdesk/\n":           true,
		"# comment\n.arcdesk/\n":   true,
	}
	for content, want := range cases {
		if got := gitignoreCoversArcDesk(content); got != want {
			t.Fatalf("gitignoreCoversArcDesk(%q) = %v, want %v", content, got, want)
		}
	}
}

func TestEnsureProjectGitignoreCreatesFile(t *testing.T) {
	root := t.TempDir()
	if err := ensureProjectGitignore(root); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, ".arcdesk/") {
		t.Fatalf("expected .arcdesk/ in new gitignore: %q", text)
	}
}

func TestEnsureProjectGitignoreAppendsWithoutDuplicate(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(path, []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureProjectGitignore(root); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, "node_modules/") {
		t.Fatalf("should preserve existing rules: %q", text)
	}
	if strings.Count(text, ".arcdesk/") != 1 {
		t.Fatalf("should append .arcdesk/ once: %q", text)
	}
	if err := ensureProjectGitignore(root); err != nil {
		t.Fatal(err)
	}
	b2, _ := os.ReadFile(path)
	if strings.Count(string(b2), ".arcdesk/") != 1 {
		t.Fatalf("second call should not duplicate .arcdesk/: %q", string(b2))
	}
}

func TestInitProjectGitRepositoryCreatesGitAndGitignore(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	app := NewApp()
	got := app.InitProjectGitRepository(root)
	if got.Err != "" {
		t.Fatalf("InitProjectGitRepository: err=%q output=%q", got.Err, got.Output)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatalf("expected .git: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), ".arcdesk/") {
		t.Fatalf("expected .arcdesk/ in gitignore: %q", string(b))
	}
}
