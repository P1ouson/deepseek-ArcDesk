package plancache

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/intent"
	"arcdesk/internal/repomap"
)

func TestPlanCacheLookupAndRecord(t *testing.T) {
	root := initGitFixture(t)
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/plancache\n\ngo 1.22\n")
	runGit(t, root, "add", "go.mod")
	runGit(t, root, "commit", "-m", "init")

	store, err := Open(root, Settings{MinPhases: 2})
	if err != nil {
		t.Fatal(err)
	}
	in := intent.Result{Class: intent.ClassRefactor, Canonical: intent.ClassRefactor, Confidence: 0.9}
	plan := "1. Audit frontend layout\n   - read main entry\n2. Refactor components\n   - grep usages\n3. Verify\n   - go test ./..."
	store.Record(in, root, plan)

	hint, hit := store.Lookup(in, root)
	if !hit {
		t.Fatal("expected cache hit after record")
	}
	if !strings.Contains(hint, "Audit frontend layout") {
		t.Fatalf("hint missing skeleton: %q", hint)
	}
	if !strings.Contains(hint, "[plan-cache hint") {
		t.Fatal("hint missing provenance tag")
	}
}

func TestPlanCacheMissOnHeadChange(t *testing.T) {
	root := initGitFixture(t)
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/plancache2\n\ngo 1.22\n")
	runGit(t, root, "add", "go.mod")
	runGit(t, root, "commit", "-m", "init")

	store, err := Open(root, Settings{MinPhases: 2})
	if err != nil {
		t.Fatal(err)
	}
	in := intent.Result{Class: intent.ClassRefactor, Canonical: intent.ClassRefactor, Confidence: 0.9}
	store.Record(in, root, "1. Step A\n2. Step B\n3. Step C")

	writeFile(t, filepath.Join(root, "README.md"), "v2\n")
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "docs")

	if _, hit := store.Lookup(in, root); hit {
		t.Fatal("expected miss after HEAD change")
	}
}

func TestPlanCacheSkipsLowConfidence(t *testing.T) {
	root := initGitFixture(t)
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/plancache3\n\ngo 1.22\n")
	runGit(t, root, "add", "go.mod")
	runGit(t, root, "commit", "-m", "init")

	store, err := Open(root, Settings{})
	if err != nil {
		t.Fatal(err)
	}
	in := intent.Result{Class: intent.ClassRefactor, Canonical: intent.ClassRefactor, Confidence: 0.5}
	store.Record(in, root, "1. A\n2. B\n3. C")
	if _, hit := store.Lookup(in, root); hit {
		t.Fatal("expected miss for low confidence")
	}
}

func TestPlanCacheTTL(t *testing.T) {
	root := initGitFixture(t)
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/plancache4\n\ngo 1.22\n")
	runGit(t, root, "add", "go.mod")
	runGit(t, root, "commit", "-m", "init")

	store, err := Open(root, Settings{TTLDays: 1, MinPhases: 2})
	if err != nil {
		t.Fatal(err)
	}
	in := intent.Result{Class: intent.ClassExplore, Canonical: intent.ClassExplore, Confidence: 0.9}
	head, _ := repomap.WorkspaceRevision(root)
	store.mu.Lock()
	store.entries[cacheKey(in.Canonical, head)] = Entry{
		Intent: in.Canonical, RepoHead: head,
		Phases:    []Phase{{Title: "A"}, {Title: "B"}},
		CreatedAt: time.Now().UTC().Add(-48 * time.Hour),
	}
	store.mu.Unlock()
	if _, hit := store.Lookup(in, root); hit {
		t.Fatal("expected TTL expiry miss")
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
