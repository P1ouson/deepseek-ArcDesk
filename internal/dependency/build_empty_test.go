package dependency

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildGraphEmptyProject(t *testing.T) {
	root := t.TempDir()
	g, meta, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	if g == nil || meta == nil {
		t.Fatal("expected empty graph and meta")
	}
	if len(g.Nodes) != 0 {
		t.Fatalf("nodes = %d, want 0", len(g.Nodes))
	}
	if g.Stats.NodeCount != 0 {
		t.Fatalf("NodeCount = %d, want 0", g.Stats.NodeCount)
	}
}

func TestIndexEmptyGraphStatus(t *testing.T) {
	root := t.TempDir()
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	stats, err := idx.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if stats.NodeCount != 0 || stats.EdgeCount != 0 {
		t.Fatalf("stats = %+v, want zero counts", stats)
	}
}

func TestEnsureReadyEmptyAfterGoModRemoved(t *testing.T) {
	root := t.TempDir()
	goMod := filepath.Join(root, "go.mod")
	if err := os.WriteFile(goMod, []byte("module empty.test\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatalf("EnsureReady with go.mod: %v", err)
	}
	if err := os.Remove(goMod); err != nil {
		t.Fatal(err)
	}
	if err := idx.RefreshIfStale(context.Background()); err != nil {
		t.Fatalf("RefreshIfStale after removing go.mod: %v", err)
	}
	stats, err := idx.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if stats.NodeCount != 0 {
		t.Fatalf("NodeCount = %d, want 0 after empty rebuild", stats.NodeCount)
	}
}
