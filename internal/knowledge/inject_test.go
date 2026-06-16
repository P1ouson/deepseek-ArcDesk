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

func TestRetryHintIncludesProvenanceMeta(t *testing.T) {
	root := t.TempDir()
	store, err := failuremem.Open(root, 10)
	if err != nil {
		t.Fatal(err)
	}
	head := "0123456789abcdef0123456789abcdef01234567"
	if err := store.Record(failuremem.Entry{
		Signature:  "go test ./internal/counter",
		Error:      "FAIL",
		Fix:        "Implement Add in counter.go to return the sum of its arguments.",
		RepoHead:   head,
		Confidence: failuremem.ConfidenceVerified,
		TS:         time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	cfg := config.KnowledgeConfig{}
	hint := RetryHint(store, cfg, RetryParams{
		FailedCmd: "go test ./internal/counter",
		Paths:     []string{"internal/counter/counter.go"},
	})
	if hint == "" {
		t.Fatal("expected hint")
	}
	if !strings.Contains(hint, "confidence=verified") {
		t.Fatalf("hint missing confidence meta: %q", hint)
	}
	if !strings.Contains(hint, "head=0123456") {
		t.Fatalf("hint missing head meta: %q", hint)
	}
}

func TestRetryHintSkipsOldCommit(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/x\n")
	runGit(t, root, "init")
	runGit(t, root, "add", "go.mod")
	runGit(t, root, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(root, ".gitkeep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := failuremem.Open(root, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Record(failuremem.Entry{
		Signature: "go test ./pkg",
		Error:     "fail",
		Fix:       "fix at old commit with enough detail to qualify",
		RepoHead:  "deadbeef00000000000000000000000000000000",
		TS:        time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	cfg := config.KnowledgeConfig{}
	hint := RetryHint(store, cfg, RetryParams{FailedCmd: "go test ./pkg"})
	if hint != "" {
		t.Fatalf("expected no hint on commit mismatch, got %q", hint)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@test", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@test")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
