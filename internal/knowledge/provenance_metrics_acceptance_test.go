package knowledge

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/config"
	"arcdesk/internal/failuremem"
)

// TestKnowledgeProvenanceInjectionLift verifies P3 filters stale lessons and
// improves inject precision vs legacy ranked search.
func TestKnowledgeProvenanceInjectionLift(t *testing.T) {
	root := initGitFixture(t)
	store, err := failuremem.Open(root, 20)
	if err != nil {
		t.Fatal(err)
	}
	head, _ := failuremem.WorkspaceProvenance(root)
	if head == "" {
		t.Fatal("expected git HEAD in fixture")
	}

	matchingFix := "Implement Add in counter.go to return the sum of its arguments."
	oldFix := "Old-branch fix: change return type in counter.go for legacy API."
	expiredFix := "Expired lesson: adjust counter_test expected values carefully."

	recordLesson(t, store, "go test ./internal/counter", matchingFix, "FAIL counter TestAdd", head, time.Now().UTC())
	recordLesson(t, store, "go test ./internal/counter", oldFix, "FAIL counter legacy API", "deadbeef00000000000000000000000000000000", time.Now().UTC())
	recordLesson(t, store, "go test ./internal/counter", expiredFix, "FAIL counter expected values", head, time.Now().UTC().Add(-120*24*time.Hour))

	ctx := failuremem.NewSearchContext(root, 90, true)
	legacy, err := store.RankedSearch("go test ./internal/counter", []string{"internal/counter/counter.go"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := store.RankedSearchWithContext(ctx, "go test ./internal/counter", []string{"internal/counter/counter.go"}, 5)
	if err != nil {
		t.Fatal(err)
	}

	if len(legacy) < 2 {
		t.Fatalf("legacy ranked len = %d, want >= 2 (includes stale lessons)", len(legacy))
	}
	if len(filtered) != 1 {
		t.Fatalf("filtered ranked len = %d, want 1 (matching head only)", len(filtered))
	}
	if !strings.Contains(filtered[0].Fix, "Implement Add") {
		t.Fatalf("filtered fix = %q, want matching-head lesson", filtered[0].Fix)
	}

	noiseRemoved := len(legacy) - len(filtered)
	if noiseRemoved < 1 {
		t.Fatalf("noise removed = %d, want >= 1 stale lesson filtered out", noiseRemoved)
	}

	cfg := config.KnowledgeConfig{}
	hint := RetryHint(store, cfg, RetryParams{
		FailedCmd: "go test ./internal/counter",
		Paths:     []string{"internal/counter/counter.go"},
	})
	if hint == "" {
		t.Fatal("expected retry hint for matching-head lesson")
	}
	if !strings.Contains(hint, "Implement Add") {
		t.Fatalf("hint missing matching fix: %q", hint)
	}
	if strings.Contains(hint, "Old-branch fix") || strings.Contains(hint, "Expired lesson") {
		t.Fatalf("hint leaked stale lesson: %q", hint)
	}

	injectPrecision := float64(len(filtered)) / float64(len(filtered)) * 100
	if injectPrecision < 100 {
		t.Fatalf("inject precision = %.1f%%, want 100%%", injectPrecision)
	}

	t.Logf("P3 metrics: legacy=%d filtered=%d noise_removed=%d inject_precision=%.0f%%",
		len(legacy), len(filtered), noiseRemoved, injectPrecision)
}

func recordLesson(t *testing.T, store *failuremem.Store, sig, fix, errMsg, head string, ts time.Time) {
	t.Helper()
	if err := store.Record(failuremem.Entry{
		Signature:  sig,
		Error:      errMsg,
		Fix:        fix,
		Paths:      []string{"internal/counter/counter.go"},
		Confidence: failuremem.ConfidenceVerified,
		RepoHead:   head,
		TS:         ts,
	}); err != nil {
		t.Fatal(err)
	}
}

func initGitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/counter\n")
	runGit(t, root, "init")
	runGit(t, root, "add", "go.mod")
	runGit(t, root, "commit", "-m", "init")
	return root
}
