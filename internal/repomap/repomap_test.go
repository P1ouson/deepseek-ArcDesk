package repomap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeEmptyWorkspace(t *testing.T) {
	if got := Compose("base", ""); got != "base" {
		t.Fatalf("got %q", got)
	}
}

func TestRefreshAndLoad(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Demo\n\nHello."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	block := LoadBlock(root)
	if block == "" {
		t.Fatal("expected block")
	}
	if !strings.Contains(block, "README") || !strings.Contains(block, "Top-level layout") {
		t.Fatalf("block missing sections: %q", block)
	}
	out := Compose("SYSTEM", root)
	if !strings.Contains(out, "SYSTEM") || !strings.Contains(out, "Project repository map") {
		t.Fatalf("compose failed: %q", out)
	}
}

func TestRecordRead(t *testing.T) {
	root := t.TempDir()
	if err := RecordRead(root, "src/main.go", "entry point"); err != nil {
		t.Fatal(err)
	}
	rows := loadReadIndex(root)
	if len(rows) != 1 || rows[0].Path != "src/main.go" {
		t.Fatalf("rows = %v", rows)
	}
	// read-index is persisted; folded into the prefix once a repo-map exists.
	if block := LoadBlock(root); block != "" {
		t.Fatalf("LoadBlock without repo-map should stay empty, got %q", block)
	}
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	block := LoadBlock(root)
	if !strings.Contains(block, "Top-level layout") {
		t.Fatalf("expected repo-map in block")
	}
	if !strings.Contains(block, "Recently read files") || !strings.Contains(block, "src/main.go") {
		t.Fatalf("read-index should appear in prefix block: %q", block)
	}
}

func TestRecordExploreSummary(t *testing.T) {
	root := t.TempDir()
	if err := RecordExploreSummary(root, "where is main?", "cmd/main.go is the entry"); err != nil {
		t.Fatal(err)
	}
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	block := LoadBlock(root)
	if !strings.Contains(block, "Explore conclusions") || !strings.Contains(block, "where is main?") {
		t.Fatalf("explore summary missing from block: %q", block)
	}
}

func TestEnsureReadyCreatesMap(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureReady(root); err != nil {
		t.Fatal(err)
	}
	mp, err := mapPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mp); err != nil {
		t.Fatalf("map not created: %v", err)
	}
}

func TestRefreshIfStaleCoalesce(t *testing.T) {
	root := t.TempDir()
	// Own git toplevel so repoRevision does not inherit a parent worktree HEAD
	// (TMPDIR inside a checkout would otherwise make isStale flaky under load).
	runGit(t, root, "init")
	if err := RefreshIfStale(root); err != nil {
		t.Fatal(err)
	}
	stale, err := isStale(root)
	if err != nil || stale {
		t.Fatalf("stale=%v err=%v", stale, err)
	}
}

func TestIsStaleStableAfterMetaWrite(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	stale, err := isStale(root)
	if err != nil || stale {
		t.Fatalf("after refresh stale=%v err=%v", stale, err)
	}
}
