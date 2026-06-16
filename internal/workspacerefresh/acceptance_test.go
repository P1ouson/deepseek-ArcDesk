package workspacerefresh

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"arcdesk/internal/config"
	"arcdesk/internal/dependency"
	"arcdesk/internal/tool"
)

// TestP4PlanMetaBumpOnDocChange verifies planning predicts meta_bump not full refresh.
func TestP4PlanMetaBumpOnDocChange(t *testing.T) {
	root := initGitFixture(t)
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/p4plan\n\ngo 1.22\n")
	writeFile(t, filepath.Join(root, "internal", "app", "app.go"), "package app\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "init")

	idx, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(root, "CHANGELOG.md"), "v0\n")
	runGit(t, root, "add", "CHANGELOG.md")
	runGit(t, root, "commit", "-m", "changelog")

	cfg := &config.Config{}
	plan := BuildPlan(PlanInput{Root: root, Cfg: cfg, Dep: idx})
	var depLayer LayerPlan
	for _, l := range plan.Layers {
		if l.Name == "dependency" {
			depLayer = l
			break
		}
	}
	if depLayer.Action != ActionMetaBump {
		t.Fatalf("dependency action = %q reason=%q, want meta_bump", depLayer.Action, depLayer.Reason)
	}
}

// TestP4WorkspaceRefreshStatusTool verifies the read-only status tool exposes reuse plan.
func TestP4WorkspaceRefreshStatusTool(t *testing.T) {
	root := initGitFixture(t)
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/p4tool\n\ngo 1.22\n")
	writeFile(t, filepath.Join(root, "internal", "x", "x.go"), "package x\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "init")

	idx, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	host := NewHost(root, &config.Config{}, idx, nil)
	reg := tool.NewRegistry()
	RegisterTools(reg, host)
	tl, ok := reg.Get("workspace_refresh_status")
	if !ok {
		t.Fatal("workspace_refresh_status not registered")
	}
	out, err := tl.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("empty status output")
	}
	var payload struct {
		Plan Plan `json:"plan"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Plan.Layers) == 0 {
		t.Fatal("expected layer plans")
	}
}

func initGitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	return root
}

func writeFile(t *testing.T, path, body string) {
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
