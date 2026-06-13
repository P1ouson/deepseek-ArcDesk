package failuremem

import (
	"context"
	"encoding/json"
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
	if _, err := record.Execute(ctx, json.RawMessage(`{"signature":"vitest ui","fix":"mock fetch"}`)); err != nil {
		t.Fatal(err)
	}
}
