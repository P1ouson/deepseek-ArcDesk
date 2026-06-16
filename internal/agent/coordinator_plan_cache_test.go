package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/event"
	"arcdesk/internal/intent"
	"arcdesk/internal/plancache"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
)

// TestP5PlanCacheInjectsSkeleton verifies cache hit prepends hint to planner input.
func TestP5PlanCacheInjectsSkeleton(t *testing.T) {
	root := initPlanCacheGit(t)
	store, err := plancache.Open(root, plancache.Settings{MinPhases: 2})
	if err != nil {
		t.Fatal(err)
	}
	in := intent.Result{Class: intent.ClassRefactor, Canonical: intent.ClassRefactor, Confidence: 0.9}
	store.Record(in, root, "1. Map module boundaries\n2. Extract shared helpers\n3. Run go test ./...")

	planner := &mockProvider{name: "planner", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "1. Map\n2. Extract\n3. Test"},
		{Type: provider.ChunkDone},
	}}
	exec := &mockProvider{name: "executor", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "done"},
		{Type: provider.ChunkDone},
	}}
	executor := New(exec, tool.NewRegistry(), NewSession("exec"), Options{}, event.Discard)
	coord := NewCoordinator(planner, NewSession("plan"), nil, executor, 0, event.Discard, nil, nil)
	coord.BindPlanCache(store, root)

	task := "帮我重构前端模块边界"
	if err := coord.Run(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	got := lastUser(planner.lastReq)
	if !strings.Contains(got, "[plan-cache hint") {
		t.Fatalf("planner input missing cache hint: %q", got)
	}
	if !strings.Contains(got, "Map module boundaries") {
		t.Fatalf("planner input missing cached skeleton: %q", got)
	}
}

func initPlanCacheGit(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGitPC(t, root, "init")
	writePC(t, filepath.Join(root, "go.mod"), "module example.com/p5\n\ngo 1.22\n")
	runGitPC(t, root, "add", "go.mod")
	runGitPC(t, root, "commit", "-m", "init")
	return root
}

func writePC(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGitPC(t *testing.T, dir string, args ...string) {
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
