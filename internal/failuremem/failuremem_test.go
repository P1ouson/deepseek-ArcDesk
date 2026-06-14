package failuremem

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestFailureMemoryRoundTrip(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Record(Entry{Signature: "go test ./pkg", Error: "undefined: Foo", Fix: "import pkg/foo"}); err != nil {
		t.Fatal(err)
	}
	hits, err := store.Search("undefined", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("search=%v err=%v", hits, err)
	}
	reg := tool.NewRegistry()
	RegisterTools(reg, store)
	ctx := context.Background()
	search, _ := reg.Get("failuremem_search")
	out, err := search.Execute(ctx, json.RawMessage(`{"query":"Foo","limit":3}`))
	if err != nil || out == "" {
		t.Fatalf("tool: %q err=%v", out, err)
	}
	record, _ := reg.Get("failuremem_record")
	if _, err := record.Execute(ctx, json.RawMessage(`{"signature":"vitest ui","fix":"mock fetch in test setup"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestRecordMergesByFingerprint(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Record(Entry{Signature: "go test ./pkg", Error: "undefined: Foo", Fix: "import pkg/foo"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Record(Entry{Signature: "go test ./pkg", Error: "undefined: Foo", Fix: "import pkg/foo with alias"}); err != nil {
		t.Fatal(err)
	}
	entries, err := store.List(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want merge", len(entries))
	}
	if entries[0].Hits < 2 {
		t.Fatalf("hits = %d", entries[0].Hits)
	}
}

func TestRankedSearchPrefersPathMatch(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root, 10)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Record(Entry{Signature: "go test ./...", Fix: "generic", Paths: []string{"other.go"}})
	_ = store.Record(Entry{Signature: "go test ./internal/counter", Error: "fail", Fix: "fix counter", Paths: []string{"internal/counter/counter.go"}})
	hits, err := store.RankedSearch("go test ./internal/counter", []string{"internal/counter/counter.go"}, 1)
	if err != nil || len(hits) != 1 {
		t.Fatalf("hits=%v err=%v", hits, err)
	}
	if !strings.Contains(hits[0].Fix, "counter") {
		t.Fatalf("got %q", hits[0].Fix)
	}
}

func TestNormalizeEntryUsesDistinctIDsForSimilarSignatures(t *testing.T) {
	a := Entry{
		Signature: "go test ./internal/counter",
		Error:     "误改期望值为 99",
		Fix:       "恢复 Add(3) 期望为 5",
		Paths:     []string{"internal/counter/counter_test.go"},
	}
	b := Entry{
		Signature: "go test ./internal/counter: 误改期望值导致测试失败",
		Error:     "误改期望值为 99",
		Fix:       "恢复 Add(3) 期望为 5",
		Paths:     []string{"internal/counter/counter_test.go"},
	}
	NormalizeEntry(&a)
	NormalizeEntry(&b)
	if a.ID == "" || b.ID == "" {
		t.Fatal("expected ids")
	}
	if a.ID == b.ID {
		t.Fatalf("colliding ids: %q", a.ID)
	}
}
