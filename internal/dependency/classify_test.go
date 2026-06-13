package dependency

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoMod(t *testing.T) {
	path := filepath.Join("testdata", "go_project", "go.mod")
	info, err := parseGoMod(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Module != "example.com/testproj" {
		t.Fatalf("module = %q", info.Module)
	}
	if len(info.Replaces) == 0 {
		t.Fatal("expected replace directive")
	}
}

func TestModulePathForImport(t *testing.T) {
	got := modulePathForImport("github.com/foo/bar/baz")
	want := "github.com/foo/bar"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStdlibGOROOTProbe(t *testing.T) {
	goroot := resolveGOROOT(".")
	if goroot == "" {
		t.Skip("GOROOT unavailable")
	}
	if !isStdlibImport("fmt", goroot) {
		t.Fatal("fmt should be stdlib under GOROOT")
	}
	if isStdlibImport("github.com/foo/bar", goroot) {
		t.Fatal("github import should not be stdlib")
	}
}

func TestStdlibHeuristicWithoutGOROOT(t *testing.T) {
	if !isStdlibImport("net/http", "") {
		t.Fatal("net/http without dot should be stdlib via heuristic")
	}
}

func TestInferImportPathFromDir(t *testing.T) {
	root := filepath.Join("testdata", "go_project")
	got := inferImportPathFromDir("example.com/testproj", root, filepath.Join(root, "internal", "alpha"))
	want := "example.com/testproj/internal/alpha"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestGoModReplaceWarning(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("testdata", "go_project"))
	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, _, err = b.Build()
	if err != nil {
		t.Fatal(err)
	}
	// manifestNodesAndEdges is exercised via Build; replace warning lands on nodes.
	_, edges, _, method, _ := b.buildWithGoList()
	_, _, warns := b.manifestNodesAndEdges(method)
	if len(warns) == 0 {
		t.Fatal("expected replace warning")
	}
	_ = edges
}

func init() {
	// Ensure testdata module is readable when tests run from package dir.
	_ = os.Getenv("GO111MODULE")
}
