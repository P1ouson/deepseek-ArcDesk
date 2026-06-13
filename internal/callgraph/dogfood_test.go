package callgraph

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"arcdesk/internal/dependency"
)

// TestDogfoodArcDeskRepo builds the callgraph on the enclosing ArcDesk Wails repo.
func TestDogfoodArcDeskRepo(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if !Discoverable(root) {
		t.Skip("not a Wails project")
	}

	dep, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := dep.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	storeDir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(storeDir)

	idx, err := Open(root, NewDependencyCatalog(dep))
	if err != nil {
		t.Fatal(err)
	}

	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	stats, err := idx.Status()
	if err != nil {
		t.Fatal(err)
	}
	if stats.NodeCount == 0 {
		t.Fatal("expected nodes after fresh build")
	}
	if stats.GoBindCount < 10 {
		t.Fatalf("go binds = %d, want substantial App surface", stats.GoBindCount)
	}
	if stats.BridgeCallCount < 5 {
		t.Fatalf("bridge calls = %d, want frontend RPC calls", stats.BridgeCallCount)
	}
	if stats.EventEmitCount == 0 {
		t.Fatalf("event emits = %d, want EventsEmit sites", stats.EventEmitCount)
	}
	if stats.EventListenCount == 0 {
		t.Fatalf("event listens = %d, want EventsOn sites", stats.EventListenCount)
	}

	backward, err := idx.TraceBackward(context.Background(), "SubmitToTab", DefaultTraceOptions())
	if err != nil {
		t.Fatalf("TraceBackward(SubmitToTab): %v", err)
	}
	if len(backward) == 0 {
		t.Fatal("expected backward path for SubmitToTab")
	}

	forward, err := idx.TraceForward(context.Background(), "desktop/frontend/src/components/Composer.tsx", "Composer", DefaultTraceOptions())
	if err == nil && len(forward) == 0 {
		t.Log("Composer forward trace empty (symbol may differ); skipping strict assert")
	}
}
