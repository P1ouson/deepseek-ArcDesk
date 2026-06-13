package dependency

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func copyGoTestProject(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("testdata", "go_project"))
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func copyDir(src, dst string) error {
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

func TestBuildGraphGoProject(t *testing.T) {
	root := copyGoTestProject(t)
	g, meta, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	if meta == nil || meta.Fingerprint == "" {
		t.Fatal("expected fingerprint in meta")
	}
	if len(g.Nodes) == 0 {
		t.Fatal("expected nodes")
	}
	if g.Stats.NodeCount != len(g.Nodes) {
		t.Fatalf("stats node count = %d, graph = %d", g.Stats.NodeCount, len(g.Nodes))
	}
}

func TestIndexOpenEnsureReady(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	stats, err := idx.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if stats.NodeCount == 0 {
		t.Fatal("expected nodes after ensure")
	}
}

func TestIndexQueries(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	gamma := NewGoID("example.com/testproj/internal/gamma")
	res, err := idx.AffectedBy(gamma)
	if err != nil {
		t.Fatalf("AffectedBy: %v", err)
	}
	if len(res.Layers.Direct) == 0 {
		t.Fatalf("expected direct importers for gamma: %+v", res)
	}

	id, err := idx.ResolveID("internal/gamma")
	if err != nil {
		t.Fatalf("ResolveID: %v", err)
	}
	if id != gamma {
		t.Fatalf("ResolveID = %q, want %q", id, gamma)
	}

	imports, err := idx.ImportsOf(NewGoID("example.com/testproj/internal/alpha"))
	if err != nil {
		t.Fatalf("ImportsOf: %v", err)
	}
	if len(imports) == 0 {
		t.Fatal("expected alpha to import something")
	}
}

func TestInvalidateFilesMarksStale(t *testing.T) {
	root := copyGoTestProject(t)
	idx, _ := Open(root)
	_ = idx.EnsureReady(context.Background())
	if err := idx.InvalidateFiles([]string{"internal/alpha/alpha.go"}); err != nil {
		t.Fatal(err)
	}
	stats, err := idx.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !stats.Stale {
		t.Fatal("expected stale after InvalidateFiles")
	}
}
