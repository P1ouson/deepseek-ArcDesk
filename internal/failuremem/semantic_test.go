package failuremem

import (
	"strings"
	"testing"
)

func TestTokenSimilarity(t *testing.T) {
	q := tokenSet("database connection pool timeout error")
	d := tokenSet("connection pool timed out db refused")
	sim := tokenSimilarity(q, d)
	if sim < 0.35 {
		t.Fatalf("similarity = %.2f, want >= 0.35", sim)
	}
}

func TestRankedSearchSmartSemanticFallback(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root, 20)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Record(Entry{
		Signature:  "pnpm test --filter ui",
		Error:      "ModuleNotFoundError: Cannot find @scope/pkg",
		Fix:        "1. Add @scope/pkg to package.json\n2. Run pnpm install\n3. Re-run tests",
		Paths:      []string{"package.json"},
		Confidence: ConfidenceVerified,
	}); err != nil {
		t.Fatal(err)
	}

	ctx := NewSearchContext(root, 90, true)
	query := "yarn workspace api suite"
	stderr := "scoped library missing from hoisted node_modules tree"
	matches, err := store.RankedSearchSmart(ctx, query+" "+stderr, []string{"package.json"}, 3, SemanticSettings{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected semantic match")
	}
	if matches[0].Kind != MatchSemantic {
		t.Fatalf("kind = %q, want semantic (score=%.2f)", matches[0].Kind, matches[0].Score)
	}
	if matches[0].Score < 0.35 {
		t.Fatalf("semantic score = %.2f, want >= 0.35", matches[0].Score)
	}
}

func TestRankedSearchSmartExactWins(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root, 20)
	if err != nil {
		t.Fatal(err)
	}
	sig := "go test ./internal/counter"
	if err := store.Record(Entry{
		Signature: sig, Error: "FAIL counter", Fix: "Implement Add in counter.go",
		Paths: []string{"internal/counter/counter.go"}, Confidence: ConfidenceVerified,
	}); err != nil {
		t.Fatal(err)
	}
	ctx := NewSearchContext(root, 90, true)
	matches, err := store.RankedSearchSmart(ctx, sig, []string{"internal/counter/counter.go"}, 1, SemanticSettings{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 || matches[0].Kind != MatchExact {
		t.Fatalf("matches = %+v, want exact hit", matches)
	}
}

func TestFixSkeleton(t *testing.T) {
	sk := FixSkeleton("1. Add dep\n2. Install\n3. Test")
	if sk == "" || !strings.Contains(sk, "1.") || !strings.Contains(sk, "2.") {
		t.Fatalf("skeleton = %q", sk)
	}
}
