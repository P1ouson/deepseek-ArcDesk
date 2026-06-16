package knowledge

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/config"
	"arcdesk/internal/failuremem"
)

// TestP6SemanticFallbackInjectionLift verifies semantic playbook fallback when exact match fails.
func TestP6SemanticFallbackInjectionLift(t *testing.T) {
	root := initGitFixtureP6(t)
	store, err := failuremem.Open(root, 20)
	if err != nil {
		t.Fatal(err)
	}
	head, _ := failuremem.WorkspaceProvenance(root)
	if head == "" {
		t.Fatal("need git HEAD")
	}

	recordLessonP6(t, store, "pnpm test --filter ui", "ModuleNotFoundError: Cannot find @scope/pkg",
		"1. Add @scope/pkg to package.json\n2. Run pnpm install\n3. Re-run tests",
		head, time.Now().UTC())

	ctx := failuremem.NewSearchContext(root, 90, true)
	// Legacy exact search may still rank path-only rows; semantic fallback uses weak TextScore gate.
	exact, _ := store.RankedSearchWithContext(ctx, "yarn workspace api test", []string{"package.json"}, 3)

	cfg := config.KnowledgeConfig{}
	hint := RetryHint(store, cfg, RetryParams{
		FailedCmd: "yarn workspace api suite",
		Stderr:    "scoped library missing from hoisted node_modules tree",
		Paths:     []string{"package.json"},
	})
	if hint == "" {
		t.Fatal("expected semantic retry hint")
	}
	if !strings.Contains(hint, "match=semantic") {
		t.Fatalf("hint missing semantic tag: %q", hint)
	}
	if !strings.Contains(hint, "Add @scope/pkg") {
		t.Fatalf("hint missing playbook skeleton: %q", hint)
	}

	matches, err := store.RankedSearchSmart(ctx, "yarn workspace api suite scoped library missing from hoisted node_modules tree", []string{"package.json"}, 1, cfg.SemanticSettings())
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 || matches[0].Kind != failuremem.MatchSemantic {
		t.Fatalf("smart search = %+v", matches)
	}
	t.Logf("P6 metrics: exact=%d semantic_score=%.2f hint_len=%d", len(exact), matches[0].Score, len(hint))
}

func recordLessonP6(t *testing.T, store *failuremem.Store, sig, errMsg, fix, head string, ts time.Time) {
	t.Helper()
	if err := store.Record(failuremem.Entry{
		Signature: sig, Error: errMsg, Fix: fix,
		Paths: []string{"package.json"},
		Confidence: failuremem.ConfidenceVerified,
		RepoHead: head, TS: ts,
	}); err != nil {
		t.Fatal(err)
	}
}

func initGitFixtureP6(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFileP6(t, filepath.Join(root, "go.mod"), "module example.com/p6\n")
	runGitP6(t, root, "init")
	runGitP6(t, root, "add", "go.mod")
	runGitP6(t, root, "commit", "-m", "init")
	return root
}

func writeFileP6(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGitP6(t *testing.T, dir string, args ...string) {
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
