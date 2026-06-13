package callgraph

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func copyWailsTestProject(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("testdata", "wails_project"))
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func TestGraphRoundTripSaveLoad(t *testing.T) {
	dir := t.TempDir()
	g := NewGraph(dir)
	g.BuiltAt = time.Now().UTC().Truncate(time.Second)
	id := NewGoBindID("desktop/app.go", "App.Submit")
	g.AddNode(&Node{ID: id, Kind: KindGoBind, Name: "App.Submit", File: "desktop/app.go"})
	bridge := NewBridgeCallID("desktop/frontend/src/lib/useSubmit.ts", 4, "Submit")
	g.AddNode(&Node{ID: bridge, Kind: KindBridgeCall, Name: "app.Submit", File: "desktop/frontend/src/lib/useSubmit.ts", Line: 4})
	g.AddEdge(bridge, id, EdgeBridgeInvoke)
	g.RebuildIndexes()
	g.Stats = computeStats(g, time.Millisecond, 0)

	if err := SaveIndex(g, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Nodes) != 2 {
		t.Fatalf("nodes = %d", len(loaded.Nodes))
	}
	if len(loaded.MethodMap) == 0 {
		t.Fatal("expected methodMap")
	}
}

func TestBuildGraphWailsProject(t *testing.T) {
	root := copyWailsTestProject(t)
	g, meta, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if meta == nil || meta.Fingerprint == "" {
		t.Fatal("expected meta fingerprint")
	}
	if g.Stats.GoBindCount < 1 {
		t.Fatalf("go binds = %d", g.Stats.GoBindCount)
	}
	if g.Stats.BridgeCallCount < 1 {
		t.Fatalf("bridge calls = %d", g.Stats.BridgeCallCount)
	}
	if _, ok := g.MethodMap["Submit"]; !ok {
		t.Fatalf("methodMap = %v", g.MethodMap)
	}
}

func TestTraceForwardBackwardWailsProject(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	forward := TraceForward(g, NewHookID("desktop/frontend/src/lib/useSubmit.ts", "useSubmit"), DefaultTraceOptions())
	if len(forward) == 0 {
		t.Fatal("expected forward path from hook to go bind")
	}
	last := forward[0].Segments[len(forward[0].Segments)-1].Node.Kind
	if last != KindGoBind {
		t.Fatalf("forward terminus = %s", last)
	}

	gobind := g.MethodMap["Submit"]
	backward := TraceBackward(g, gobind, DefaultTraceOptions())
	if len(backward) == 0 {
		t.Fatal("expected backward path from go bind")
	}
}

func TestDiscoverable(t *testing.T) {
	root := copyWailsTestProject(t)
	if !Discoverable(root) {
		t.Fatal("wails test project should be discoverable")
	}
	if Discoverable(t.TempDir()) {
		t.Fatal("empty dir should not be discoverable")
	}
}