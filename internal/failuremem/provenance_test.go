package failuremem

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordStampsProvenance(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := Open(root, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Record(Entry{Signature: "go test ./pkg", Error: "fail", Fix: "import pkg/foo"}); err != nil {
		t.Fatal(err)
	}
	entries, err := store.List(1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	if entries[0].WorkspaceFingerprint == "" && entries[0].RepoHead == "" {
		t.Fatalf("expected provenance stamp, got %+v", entries[0])
	}
}

func TestRankedSearchWithContextSkipsCommitMismatch(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Record(Entry{
		Signature: "go test ./pkg",
		Error:     "fail",
		Fix:       "fix at old commit",
		RepoHead:  "deadbeef00000000000000000000000000000000",
	}); err != nil {
		t.Fatal(err)
	}
	ctx := SearchContext{
		RepoHead:            "cafebab000000000000000000000000000000000",
		TTLDays:             90,
		RequireMatchingHead: true,
		Now:                 time.Now().UTC(),
	}
	hits, err := store.RankedSearchWithContext(ctx, "go test", nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected no injectable hits on head mismatch, got %+v", hits)
	}
}

func TestRankedSearchWithContextAllowsMatchingHead(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root, 10)
	if err != nil {
		t.Fatal(err)
	}
	head := "abc123def4567890abcdef1234567890abcdef12"
	if err := store.Record(Entry{
		Signature: "go test ./pkg",
		Error:     "fail",
		Fix:       "fix counter Add return value",
		RepoHead:  head,
	}); err != nil {
		t.Fatal(err)
	}
	ctx := SearchContext{
		RepoHead:            head,
		TTLDays:             90,
		RequireMatchingHead: true,
		Now:                 time.Now().UTC(),
	}
	hits, err := store.RankedSearchWithContext(ctx, "go test", nil, 3)
	if err != nil || len(hits) != 1 {
		t.Fatalf("hits=%v err=%v", hits, err)
	}
}

func TestProvenanceStatusExpired(t *testing.T) {
	e := Entry{
		Signature: "go test",
		Fix:       "fix it properly with concrete steps",
		TS:        time.Now().UTC().Add(-100 * 24 * time.Hour),
	}
	ctx := SearchContext{TTLDays: 90, Now: time.Now().UTC(), RequireMatchingHead: false}
	st := e.ProvenanceStatus(ctx)
	if st.AutoInjectable {
		t.Fatalf("expected expired entry to be non-injectable, got %+v", st)
	}
}
