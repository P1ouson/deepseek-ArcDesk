package boot

import (
	"context"
	"testing"

	"arcdesk/internal/callgraph"
)

func TestBridgeImpactAdapterNil(t *testing.T) {
	if got := newBridgeImpactAdapter(nil); got != nil {
		t.Fatal("expected nil adapter")
	}
}

func TestBridgeImpactAdapterWailsProject(t *testing.T) {
	root := copyBootCallgraphProject(t)
	idx, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	adapter := newBridgeImpactAdapter(idx)
	if adapter == nil || !adapter.Available() {
		t.Fatal("expected available bridge impact adapter")
	}
	entries, err := adapter.AffectedUI("Submit")
	if err != nil {
		t.Fatalf("AffectedUI: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected cross-realm impact entries")
	}
}

func TestBridgeImpactAdapterUnavailableOnEmptyProject(t *testing.T) {
	dir := t.TempDir()
	idx, err := callgraph.Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	adapter := newBridgeImpactAdapter(idx)
	if adapter == nil {
		t.Fatal("expected adapter")
	}
	if adapter.Available() {
		t.Fatal("empty project should not have bridge impact data")
	}
}
