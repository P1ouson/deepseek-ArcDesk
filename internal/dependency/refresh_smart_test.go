package dependency

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestRefreshIfStaleMetaBumpOnDocOnlyCommit verifies Phase-4 partial reuse:
// a commit that only changes non-module files bumps git head without rebuilding.
func TestRefreshIfStaleMetaBumpOnDocOnlyCommit(t *testing.T) {
	root := initDepGitFixture(t)
	writeGitFile(t, filepath.Join(root, "go.mod"), "module example.com/p4\n\ngo 1.22\n")
	writeGitFile(t, filepath.Join(root, "internal", "alpha", "alpha.go"), "package alpha\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "init")

	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	before, err := idx.Status()
	if err != nil {
		t.Fatal(err)
	}
	if before.NodeCount == 0 {
		t.Fatal("expected nodes after initial build")
	}
	oldMeta := idx.MetaSnapshot()
	if oldMeta == nil || oldMeta.GitHead == "" {
		t.Fatal("expected git head in meta")
	}

	writeGitFile(t, filepath.Join(root, "README.md"), "# docs only\n")
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "docs")

	if err := idx.RefreshIfStale(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, err := idx.Status()
	if err != nil {
		t.Fatal(err)
	}
	if after.NodeCount != before.NodeCount {
		t.Fatalf("node count changed %d -> %d, want meta bump without rebuild", before.NodeCount, after.NodeCount)
	}
	newMeta := idx.MetaSnapshot()
	if newMeta == nil || newMeta.GitHead == oldMeta.GitHead {
		t.Fatalf("git head not bumped: old=%q new=%q", oldMeta.GitHead, metaHead(newMeta))
	}
	if after.Stale {
		t.Fatal("index should be fresh after meta bump")
	}
}

func metaHead(m *Meta) string {
	if m == nil {
		return ""
	}
	return m.GitHead
}

func initDepGitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	return root
}

func writeGitFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@test",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@test",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
