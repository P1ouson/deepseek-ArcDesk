package dependency

import (
	"context"
	"testing"
)

func TestModuleCatalogResolveFile(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	cat := idx.ModuleCatalog()
	if err := cat.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	id, ok := cat.ResolveFile("internal/alpha/alpha.go")
	if !ok {
		t.Fatal("ResolveFile alpha.go not found")
	}
	if id != string(NewGoID("example.com/testproj/internal/alpha")) {
		t.Fatalf("ResolveFile = %q", id)
	}

	kind, ok := cat.ModuleKind(id)
	if !ok || kind != "go" {
		t.Fatalf("ModuleKind = (%q, %v), want go", kind, ok)
	}
}

func TestModuleCatalogStatus(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	cat := idx.ModuleCatalog()
	if err := cat.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	nodes, edges, method := cat.Status()
	if nodes == 0 || edges == 0 {
		t.Fatalf("Status = nodes=%d edges=%d method=%q", nodes, edges, method)
	}
}
