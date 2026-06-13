package dependency

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestAffectedByLayers(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "go_project"))
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}

	gamma := NewGoID("example.com/testproj/internal/gamma")
	res, err := AffectedBy(g, gamma)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Layers.Direct) == 0 {
		t.Fatalf("expected beta as direct importer: %+v", res)
	}
	foundBeta := false
	for _, e := range res.Layers.Direct {
		if e.Name == "example.com/testproj/internal/beta" {
			foundBeta = true
		}
	}
	if !foundBeta {
		t.Fatalf("direct layers = %+v", res.Layers.Direct)
	}
}

func TestComputeImpactExternalDeps(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("example.com/a")
	ext := NewGoModID("github.com/foo/bar")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a"})
	g.AddNode(&Node{ID: ext, Kind: KindExternalGo, Name: "github.com/foo/bar"})
	g.AddEdge(a, ext, EdgeSourceImport, "a.go")

	layers := computeImpactFor(g, a)
	if len(layers.External) != 1 || layers.External[0] != ext {
		t.Fatalf("external = %+v", layers.External)
	}
}

func TestImpactCrossRealmEmpty(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/a")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "a"})
	g.Impact[id] = ImpactLayers{}
	res, err := AffectedBy(g, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CrossRealm) != 0 {
		t.Fatalf("CrossRealm = %+v, want empty", res.CrossRealm)
	}
}

func TestImpactCrossRealmJSON(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/a")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "a"})
	g.Impact[id] = ImpactLayers{}
	res, err := AffectedBy(g, id)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"crossRealm":[]`) {
		t.Fatalf("JSON = %s, want crossRealm empty array not null", b)
	}
}
