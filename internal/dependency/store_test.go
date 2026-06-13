package dependency

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGraphRoundTripSaveLoad(t *testing.T) {
	dir := t.TempDir()

	g := NewGraph(dir)
	g.BuiltAt = time.Now().UTC().Truncate(time.Second)
	g.BuildMethod = BuildGoList

	a := NewGoID("example.com/a")
	b := NewGoID("example.com/b")
	c := NewGoID("example.com/c")
	std := NewStdlibID("fmt")
	ext := NewGoModID("github.com/foo/bar")

	for _, n := range []*Node{
		{ID: a, Kind: KindInternalGo, Name: "example.com/a", Dir: "a"},
		{ID: b, Kind: KindInternalGo, Name: "example.com/b", Dir: "b"},
		{ID: c, Kind: KindInternalGo, Name: "example.com/c", Dir: "c"},
		{ID: std, Kind: KindStdlib, Name: "fmt"},
		{ID: ext, Kind: KindExternalGo, Name: "github.com/foo/bar"},
	} {
		g.AddNode(n)
	}

	g.AddEdge(a, b, EdgeSourceImport, "a/a.go")
	g.AddEdge(b, c, EdgeSourceImport, "b/b.go")
	g.AddEdge(c, std, EdgeSourceImport, "c/c.go")
	g.AddEdge(a, ext, EdgeManifestRequire, "go.mod")
	g.Impact[a] = ImpactLayers{Direct: []NodeID{b}}
	g.Files["a/a.go"] = a
	g.Stats = Stats{NodeCount: 5, EdgeCount: 4}

	if err := SaveIndex(g, dir); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}

	loaded, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	if len(loaded.Nodes) != 5 {
		t.Fatalf("nodes = %d, want 5", len(loaded.Nodes))
	}
	if len(loaded.Out[a]) != 2 {
		t.Fatalf("Out[a] = %v, want 2 deps", loaded.Out[a])
	}
	if len(loaded.In[b]) != 1 || loaded.In[b][0] != a {
		t.Fatalf("In[b] = %v, want [%s]", loaded.In[b], a)
	}
	if got := loaded.Impact[a].Direct; len(got) != 1 || got[0] != b {
		t.Fatalf("Impact[a].Direct = %v", got)
	}
	if loaded.Files["a/a.go"] != a {
		t.Fatalf("Files map = %v", loaded.Files)
	}
	got, ok := findEdge(loaded.edges, a, b, EdgeSourceImport)
	if !ok {
		t.Fatal("missing restored source import edge a->b")
	}
	if len(got.Files) != 1 || got.Files[0] != "a/a.go" {
		t.Fatalf("edge files = %v, want [a/a.go]", got.Files)
	}
	gotManifest, ok := findEdge(loaded.edges, a, ext, EdgeManifestRequire)
	if !ok || gotManifest.Kind != EdgeManifestRequire {
		t.Fatalf("missing manifest edge a->ext in %+v", loaded.edges)
	}
}

func TestLoadIndexMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadIndex(dir)
	if err != ErrIndexNotFound {
		t.Fatalf("LoadIndex() err = %v, want ErrIndexNotFound", err)
	}
}

func TestLoadIndexEmptyDir(t *testing.T) {
	_, err := LoadIndex("")
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestSaveIndexAtomicWriteFailure(t *testing.T) {
	dir := t.TempDir()
	readOnly := filepath.Join(dir, "readonly")
	if err := os.Mkdir(readOnly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o755) })

	g := NewGraph(dir)
	g.AddNode(&Node{ID: NewGoID("example.com/x"), Kind: KindInternalGo, Name: "x"})

	err := SaveIndex(g, readOnly)
	if err == nil {
		t.Skip("platform allowed write to read-only directory")
	}
	if _, statErr := os.Stat(filepath.Join(readOnly, indexFileName)); statErr == nil {
		t.Fatal("index.json should not exist after failed atomic write")
	}
}

func TestMetaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	meta := &Meta{
		GeneratedAt:  time.Now().UTC().Truncate(time.Second),
		GitHead:      "abc123",
		Fingerprint:  "fp-deadbeef",
		IndexVersion: IndexVersion,
	}
	if err := SaveMeta(meta, dir); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}
	got, err := LoadMeta(dir)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if got.Fingerprint != meta.Fingerprint || got.GitHead != meta.GitHead {
		t.Fatalf("LoadMeta = %+v, want %+v", got, meta)
	}
}

func TestGraphRemoveNode(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("a")
	b := NewGoID("b")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "b"})
	g.AddEdge(a, b, EdgeSourceImport, "a.go")

	g.RemoveNode(a)

	if _, ok := g.Nodes[a]; ok {
		t.Fatal("node a should be removed")
	}
	if len(g.In[b]) != 0 {
		t.Fatalf("In[b] = %v, want empty", g.In[b])
	}
}

func TestNodeIDHelpers(t *testing.T) {
	cases := []struct {
		id     NodeID
		realm  string
		path   string
	}{
		{NewGoID("arcdesk/internal/agent"), "go", "arcdesk/internal/agent"},
		{NewStdlibID("fmt"), "gomod", "std:fmt"},
		{NewJSID("desktop/frontend/src/lib"), "js", "desktop/frontend/src/lib"},
		{NewNpmID("react"), "npm", "react"},
	}
	for _, tc := range cases {
		if tc.id.Realm() != tc.realm {
			t.Fatalf("%s Realm() = %q, want %q", tc.id, tc.id.Realm(), tc.realm)
		}
		if tc.id.Path() != tc.path {
			t.Fatalf("%s Path() = %q, want %q", tc.id, tc.id.Path(), tc.path)
		}
		parsed, err := ParseNodeID(tc.id.String())
		if err != nil || parsed != tc.id {
			t.Fatalf("ParseNodeID(%q) = (%q, %v)", tc.id, parsed, err)
		}
	}
}
