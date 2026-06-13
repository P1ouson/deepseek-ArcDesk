package dependency

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func testdataGoProject(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "go_project")
}

func TestNewGoBuilderMissingMod(t *testing.T) {
	dir := t.TempDir()
	_, err := NewGoBuilder(dir)
	if err == nil {
		t.Fatal("expected error without go.mod")
	}
}

func TestGoBuilderBuildGoList(t *testing.T) {
	root, err := filepath.Abs(testdataGoProject(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	nodes, edges, _, method, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if method != BuildGoList {
		t.Fatalf("method = %q, want go_list", method)
	}

	kinds := kindCounts(nodes)
	if kinds[KindInternalGo] < 3 {
		t.Fatalf("internal packages = %d, want >= 3: %#v", kinds[KindInternalGo], kinds)
	}
	if kinds[KindStdlib] < 1 {
		t.Fatalf("stdlib packages = %d, want >= 1", kinds[KindStdlib])
	}
	if kinds[KindExternalGo] < 1 {
		t.Fatalf("external modules = %d, want >= 1", kinds[KindExternalGo])
	}

	if !hasEdge(edges, NewGoID("example.com/testproj/internal/alpha"), NewStdlibID("fmt")) &&
		!hasEdge(edges, NewGoID("example.com/testproj/internal/alpha"), NewGoID("fmt")) {
		// fmt may be classified via gomod:std:fmt
		found := false
		for _, e := range edges {
			if e.From == NewGoID("example.com/testproj/internal/alpha") && e.To.Realm() == realmGoMod && e.To.Path() == "std:fmt" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing alpha -> fmt edge in %#v", edges)
		}
	}

	if !hasEdge(edges, NewGoID("example.com/testproj/internal/beta"), NewGoModID("github.com/example/extpkg")) {
		t.Fatalf("missing beta -> external edge")
	}
}

func TestGoBuilderParserFallback(t *testing.T) {
	root, err := filepath.Abs(testdataGoProject(t))
	if err != nil {
		t.Fatal(err)
	}

	orig := goListRunner
	goListRunner = func(context.Context, string) ([]goListPackage, error) {
		return nil, os.ErrInvalid
	}
	t.Cleanup(func() { goListRunner = orig })

	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	nodes, edges, _, method, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if method != BuildParserFallback {
		t.Fatalf("method = %q, want parser_fallback", method)
	}
	kinds := kindCounts(nodes)
	if kinds[KindInternalGo] < 3 {
		t.Fatalf("internal = %d, want >= 3", kinds[KindInternalGo])
	}
	if kinds[KindStdlib] < 1 {
		t.Fatalf("stdlib = %d, want >= 1", kinds[KindStdlib])
	}
	if kinds[KindExternalGo] < 1 {
		t.Fatalf("external = %d, want >= 1", kinds[KindExternalGo])
	}
	if len(edges) == 0 {
		t.Fatal("expected edges from parser fallback")
	}
	for _, n := range nodes {
		if n.Meta.BuildMethod != string(BuildParserFallback) {
			continue
		}
	}
}

func TestGoBuilderRecordsSyntaxErrorFromGoList(t *testing.T) {
	root := copyGoProjectForTest(t)
	broken := filepath.Join(root, "internal", "alpha", "broken.go")
	if err := os.WriteFile(broken, []byte("package alpha\n\nfunc broken( {\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, parseErrors, method, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if method != BuildGoList {
		t.Fatalf("method = %q, want go_list", method)
	}
	if len(parseErrors) == 0 {
		t.Fatal("expected parse errors for syntax-broken file")
	}
	foundBroken := false
	for _, pe := range parseErrors {
		if strings.Contains(pe.File, "broken.go") {
			foundBroken = true
			break
		}
	}
	if !foundBroken {
		t.Fatalf("parseErrors = %+v, want broken.go entry", parseErrors)
	}

	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	if len(g.ParseErrors) == 0 {
		t.Fatal("BuildGraph.ParseErrors empty, want syntax error recorded")
	}
}

func TestGoBuilderRecordsGoListPackageError(t *testing.T) {
	root, err := filepath.Abs(testdataGoProject(t))
	if err != nil {
		t.Fatal(err)
	}
	orig := goListRunner
	goListRunner = func(context.Context, string) ([]goListPackage, error) {
		return []goListPackage{{
			ImportPath: "example.com/testproj/internal/bad",
			Dir:        filepath.Join(root, "internal", "alpha"),
			Error: &goListError{
				Pos: "internal/alpha/bad.go:3:1",
				Err: "syntax error: unexpected newline",
			},
		}}, nil
	}
	t.Cleanup(func() { goListRunner = orig })

	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, parseErrors, _, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(parseErrors) == 0 {
		t.Fatal("expected package-level go list error in ParseErrors")
	}
	if parseErrors[0].Line != 3 {
		t.Fatalf("Line = %d, want 3", parseErrors[0].Line)
	}
}

func TestGoBuilderRecordsGoListDepsErrors(t *testing.T) {
	root, err := filepath.Abs(testdataGoProject(t))
	if err != nil {
		t.Fatal(err)
	}
	orig := goListRunner
	goListRunner = func(context.Context, string) ([]goListPackage, error) {
		return []goListPackage{{
			ImportPath: "example.com/testproj/internal/bad",
			Dir:        filepath.Join(root, "internal", "alpha"),
			Imports:    []string{"nonexistent/pkg"},
			DepsErrors: []goListError{{
				Pos: "internal/alpha/alpha.go:2:8",
				Err: "package nonexistent/pkg is not in std",
			}},
		}}, nil
	}
	t.Cleanup(func() { goListRunner = orig })

	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, parseErrors, _, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(parseErrors) == 0 {
		t.Fatal("expected DepsErrors in ParseErrors")
	}
}

func TestSplitGoListPos(t *testing.T) {
	file, line := splitGoListPos("internal/alpha/broken.go:3:1")
	if file != "internal/alpha/broken.go" || line != 3 {
		t.Fatalf("splitGoListPos() = (%q, %d)", file, line)
	}
}

func copyGoProjectForTest(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(testdataGoProject(t))
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := copyDirForTest(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func copyDirForTest(src, dst string) error {
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
		return copyFileForTest(path, target)
	})
}

func copyFileForTest(src, dst string) error {
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

func TestParseGoImports(t *testing.T) {
	root := testdataGoProject(t)
	imports, err := ParseGoImports(filepath.Join(root, "internal", "alpha", "alpha.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(imports, "fmt") {
		t.Fatalf("imports = %v, want fmt", imports)
	}
	if !slices.Contains(imports, "example.com/testproj/internal/beta") {
		t.Fatalf("imports = %v, want internal beta", imports)
	}
}

func TestClassifyImportStdlibHeuristic(t *testing.T) {
	id, kind := classifyImport("example.com/mod", "fmt", "", false)
	if kind != KindStdlib || id != NewStdlibID("fmt") {
		t.Fatalf("classify fmt = (%q, %q)", id, kind)
	}
}

func kindCounts(nodes []*Node) map[Kind]int {
	out := map[Kind]int{}
	for _, n := range nodes {
		out[n.Kind]++
	}
	return out
}

func hasEdge(edges []Edge, from, to NodeID) bool {
	for _, e := range edges {
		if e.From == from && e.To == to {
			return true
		}
	}
	return false
}
