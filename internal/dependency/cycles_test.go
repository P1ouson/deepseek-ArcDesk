package dependency

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)
func TestComputeCyclesDetectsGoRing(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("example.com/a")
	b := NewGoID("example.com/b")
	c := NewGoID("example.com/c")
	for _, id := range []NodeID{a, b, c} {
		g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: id.Path()})
	}
	g.AddEdge(a, b, EdgeSourceImport, "a.go")
	g.AddEdge(b, c, EdgeSourceImport, "b.go")
	g.AddEdge(c, a, EdgeSourceImport, "c.go")

	cycles := computeCycles(g)
	if len(cycles) != 1 {
		t.Fatalf("cycles = %d, want 1", len(cycles))
	}
	if cycles[0].Lang != "go" || len(cycles[0].Ring) != 3 {
		t.Fatalf("cycle = %+v", cycles[0])
	}
	if cycles[0].Severity != "error" {
		t.Fatalf("severity = %q, want error for non-hub ring", cycles[0].Severity)
	}
}

func TestComputeCyclesIgnoresSelfLoops(t *testing.T) {
	g := NewGraph("/tmp")
	root := NewJSID("frontend/src/components")
	dock := NewJSID("frontend/src/components/dock")
	lib := NewJSID("frontend/src/lib")
	for _, id := range []NodeID{root, dock, lib} {
		g.AddNode(&Node{ID: id, Kind: KindInternalJS, Name: string(id)})
	}
	// Acyclic tree plus barrel self-imports should not report a cycle.
	g.AddEdge(root, root, EdgeSourceImport, "index.ts")
	g.AddEdge(root, dock, EdgeSourceImport, "index.ts")
	g.AddEdge(dock, lib, EdgeSourceImport, "Dock.tsx")
	g.AddEdge(lib, lib, EdgeSourceImport, "util.ts")

	cycles := computeCycles(g)
	if len(cycles) != 0 {
		t.Fatalf("self-imports on acyclic graph should not create cycles, got %+v", cycles)
	}
}

func TestComputeCyclesBarrelHubWarning(t *testing.T) {
	g := NewGraph("/tmp")
	components := NewJSID("desktop/frontend/src/components")
	dock := NewJSID("desktop/frontend/src/components/dock")
	editors := NewJSID("desktop/frontend/src/components/editors")
	settings := NewJSID("desktop/frontend/src/components/settings")
	lib := NewJSID("desktop/frontend/src/lib")
	for _, n := range []*Node{
		{ID: components, Kind: KindInternalJS, Name: "components"},
		{ID: dock, Kind: KindInternalJS, Name: "dock"},
		{ID: editors, Kind: KindInternalJS, Name: "editors"},
		{ID: settings, Kind: KindInternalJS, Name: "settings"},
		{ID: lib, Kind: KindInternalJS, Name: "lib"},
	} {
		g.AddNode(n)
	}
	// ArcDesk frontend barrel hub (no self-loops).
	g.AddEdge(components, dock, EdgeSourceImport, "index.ts")
	g.AddEdge(components, editors, EdgeSourceImport, "index.ts")
	g.AddEdge(components, settings, EdgeSourceImport, "index.ts")
	g.AddEdge(components, lib, EdgeSourceImport, "index.ts")
	g.AddEdge(dock, components, EdgeSourceImport, "Dock.tsx")
	g.AddEdge(dock, lib, EdgeSourceImport, "Dock.tsx")
	g.AddEdge(editors, components, EdgeSourceImport, "HljsCode.tsx")
	g.AddEdge(editors, lib, EdgeSourceImport, "DiffRows.tsx")
	g.AddEdge(settings, components, EdgeSourceImport, "GeneralSection.tsx")
	g.AddEdge(settings, lib, EdgeSourceImport, "GeneralSection.tsx")
	g.AddEdge(lib, components, EdgeSourceImport, "util.ts")

	cycles := computeCycles(g)
	if len(cycles) != 1 {
		t.Fatalf("cycles = %d, want 1", len(cycles))
	}
	c := cycles[0]
	if c.Severity != "warning" {
		t.Fatalf("severity = %q, want warning", c.Severity)
	}
	if c.Hub != components {
		t.Fatalf("hub = %q, want %q", c.Hub, components)
	}
	if !strings.Contains(c.Hint, "barrel hub pattern detected") {
		t.Fatalf("hint = %q, want barrel hub note", c.Hint)
	}
	if !strings.Contains(c.Hint, "components") {
		t.Fatalf("hint = %q, want hub name", c.Hint)
	}
}

func TestArcDeskBarrelHubCycleWarning(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Skip("ArcDesk module root not found")
	}
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.InvalidateFiles(nil); err != nil {
		t.Fatal(err)
	}
	if err := idx.RefreshIfStale(context.Background()); err != nil {
		t.Fatal(err)
	}
	cycles, err := idx.FindCycles()
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) != 1 {
		t.Fatalf("cycles = %d, want 1 ArcDesk JS barrel SCC", len(cycles))
	}
	c := cycles[0]
	if c.Severity != "warning" {
		t.Fatalf("severity = %q, want warning", c.Severity)
	}
	if c.Hub == "" {
		t.Fatalf("cycle = %+v, want hub set", c)
	}
	if !strings.Contains(string(c.Hub), "components") {
		t.Fatalf("hub = %q, want components package", c.Hub)
	}
	if !strings.Contains(c.Hint, "barrel hub pattern detected") {
		t.Fatalf("hint = %q", c.Hint)
	}
}

func TestComputeCyclesIgnoresManifestEdges(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("example.com/a")
	ext := NewGoModID("github.com/foo/bar")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a"})
	g.AddNode(&Node{ID: ext, Kind: KindExternalGo, Name: "github.com/foo/bar"})
	g.AddEdge(a, ext, EdgeManifestRequire, "go.mod")

	cycles := computeCycles(g)
	if len(cycles) != 0 {
		t.Fatalf("expected no cycles, got %+v", cycles)
	}
}

func TestIndexFindCycles(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("example.com/x")
	b := NewGoID("example.com/y")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "x"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "y"})
	g.AddEdge(a, b, EdgeSourceImport, "x.go")
	g.AddEdge(b, a, EdgeSourceImport, "y.go")
	g.Cycles = computeCycles(g)

	idx := &Index{graph: g}
	cycles, err := idx.FindCycles()
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) != 1 {
		t.Fatalf("FindCycles = %d", len(cycles))
	}
}
