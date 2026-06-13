package gitrag

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"arcdesk/internal/tool"
)

func TestRepoBlameAndLog(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	path := filepath.Join(dir, "pkg", "main.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, dir, "add", "pkg/main.go")
	runGitCmd(t, dir, "commit", "-m", "add main")

	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	lines, err := repo.Blame(ctx, "pkg/main.go", 1, 2, 10)
	if err != nil || len(lines) != 2 {
		t.Fatalf("blame = %v err=%v", lines, err)
	}
	commits, err := repo.Log(ctx, "pkg/main.go", 5, "")
	if err != nil || len(commits) == 0 {
		t.Fatalf("log = %v err=%v", commits, err)
	}
	if _, err := repo.ShowCommit(ctx, commits[0].Hash); err != nil {
		t.Fatal(err)
	}
}

func TestGitToolsExecute(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, dir, "add", "README.md")
	runGitCmd(t, dir, "commit", "-m", "readme")
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	reg := tool.NewRegistry()
	RegisterTools(reg, repo)
	ctx := context.Background()
	if _, ok := reg.Get("git_status"); !ok {
		t.Fatal("missing git_status")
	}
	statusTool, _ := reg.Get("git_status")
	if _, err := statusTool.Execute(ctx, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	blameTool, _ := reg.Get("git_blame")
	if _, err := blameTool.Execute(ctx, json.RawMessage(`{"path":"README.md","start_line":1,"end_line":1}`)); err != nil {
		t.Fatal(err)
	}
	logTool, _ := reg.Get("git_log")
	if _, err := logTool.Execute(ctx, json.RawMessage(`{"path":"README.md","limit":3}`)); err != nil {
		t.Fatal(err)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGitCmd(t, dir, "init")
	runGitCmd(t, dir, "config", "user.email", "t@example.com")
	runGitCmd(t, dir, "config", "user.name", "test")
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
