package dependency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"arcdesk/internal/tool"
)

type mockBridgeAnalyzer struct {
	ok      bool
	entries []CrossRealmImpactEntry
	err     error
}

func (m mockBridgeAnalyzer) Available() bool { return m.ok }

func (m mockBridgeAnalyzer) AffectedUI(string) ([]CrossRealmImpactEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.entries, nil
}

func TestCrossRealmEntriesViaIndex(t *testing.T) {
	g := NewGraph(t.TempDir())
	id := NewGoID("example.com/desktop")
	g.AddNode(&Node{
		ID:   id,
		Kind: KindInternalGo,
		Name: "desktop",
		Meta: NodeMeta{BridgeMethod: "Submit"},
	})
	g.Impact[id] = ImpactLayers{}

	idx, _ := Open(t.TempDir())
	idx.SetBridgeImpactAnalyzer(mockBridgeAnalyzer{
		ok: true,
		entries: []CrossRealmImpactEntry{
			{ID: "ui:Composer", Name: "Composer", Kind: "ui_component"},
		},
	})

	res, err := AffectedByWithAnalyzer(g, id, mockBridgeAnalyzer{
		ok: true,
		entries: []CrossRealmImpactEntry{
			{ID: "ui:Composer", Name: "Composer", Kind: "ui_component"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CrossRealm) != 1 || res.CrossRealm[0].Name != "Composer" {
		t.Fatalf("CrossRealm = %+v", res.CrossRealm)
	}
	_ = idx
}

func TestBridgeMethodsForNodeKindBridge(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("bridge:Submit")
	g.AddNode(&Node{ID: id, Kind: KindBridge, Name: "Submit"})
	methods := bridgeMethodsForNode(g, id)
	if len(methods) != 1 || methods[0] != "Submit" {
		t.Fatalf("methods = %v", methods)
	}
}

func TestFormatImpactNamesTruncation(t *testing.T) {
	entries := make([]ImpactEntry, 8)
	for i := range entries {
		entries[i] = ImpactEntry{Name: "mod" + string(rune('a'+i))}
	}
	got := formatImpactNames(entries, 3)
	if !strings.Contains(got, "…") {
		t.Fatalf("expected truncation ellipsis, got %q", got)
	}
	if got := formatImpactNames([]ImpactEntry{{ID: "x", Name: ""}}, 5); got == "" {
		t.Fatal("expected id fallback name")
	}
}

func TestFmtHintWithExternal(t *testing.T) {
	h := fmtHint("alpha", 2, 5, 1)
	if !strings.Contains(h, "external dep") {
		t.Fatalf("hint = %q", h)
	}
}

func TestIndexImportedByAndVersionConflicts(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	gamma := NewGoID("example.com/testproj/internal/gamma")
	by, err := idx.ImportedBy(gamma)
	if err != nil {
		t.Fatal(err)
	}
	if len(by) == 0 {
		t.Fatal("expected importers of gamma")
	}

	conflicts, err := idx.VersionConflicts()
	if err != nil {
		t.Fatal(err)
	}
	_ = conflicts // may be empty in test project
}

func TestIndexRootNodeNameKind(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if idx.Root() != root {
		t.Fatalf("Root() = %q", idx.Root())
	}
	_ = idx.EnsureReady(context.Background())
	gamma := NewGoID("example.com/testproj/internal/gamma")
	if name := idx.NodeName(gamma); name == "" {
		t.Fatal("expected node name")
	}
	if kind := idx.NodeKind(gamma); kind != KindInternalGo {
		t.Fatalf("kind = %q", kind)
	}
}

func TestIndexStatusNotReady(t *testing.T) {
	idx := &Index{root: t.TempDir()}
	if _, err := idx.Status(); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveIDAmbiguousAndMissing(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("example.com/a")
	b := NewGoID("example.com/b")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "dup", Dir: "pkg/a"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "dup", Dir: "pkg/b"})
	if _, err := resolveID(g, "dup"); err == nil {
		t.Fatal("expected ambiguous error")
	}
	if _, err := resolveID(g, "no-such-module"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestAffectedByMissingNode(t *testing.T) {
	g := NewGraph("/tmp")
	if _, err := AffectedBy(g, NewGoID("missing")); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestComputeImpactEmptyGraph(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("leaf")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "leaf"})
	computeImpact(g)
	res, err := AffectedBy(g, id)
	if err != nil {
		t.Fatal(err)
	}
	if res.Hint == "" {
		t.Fatal("expected hint for leaf")
	}
	if len(res.Layers.Direct) != 0 {
		t.Fatalf("direct = %+v", res.Layers.Direct)
	}
}

func TestGraphImportedBy(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("a")
	b := NewGoID("b")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "b"})
	g.AddEdge(a, b, EdgeSourceImport, "a.go")
	if importers := g.ImportedBy(b); len(importers) != 1 || importers[0] != a {
		t.Fatalf("ImportedBy = %v", importers)
	}
}

func TestRefreshIfStaleFastPath(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := idx.RefreshIfStale(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshIfStaleContextCancel(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = idx.InvalidateFiles([]string{"internal/alpha/alpha.go"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := idx.RefreshIfStale(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestRefreshConcurrentTryLock(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = idx.InvalidateFiles([]string{"internal/alpha/alpha.go"})
	mu := refreshLockFor(idx.root)
	mu.Lock()
	defer mu.Unlock()
	if err := idx.RefreshIfStale(context.Background()); err != nil {
		t.Fatalf("TryLock path should return nil: %v", err)
	}
}

func TestInvalidateFilesIdempotent(t *testing.T) {
	root := copyGoTestProject(t)
	idx, _ := Open(root)
	_ = idx.EnsureReady(context.Background())
	_ = idx.InvalidateFiles([]string{"a.go"})
	_ = idx.InvalidateFiles([]string{"b.go"})
	stats, _ := idx.Status()
	if !stats.Stale {
		t.Fatal("expected stale")
	}
}

func TestInvalidateFilesNilIndex(t *testing.T) {
	var idx *Index
	if err := idx.InvalidateFiles(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildFailureContextEmptyPathsAndLongStderr(t *testing.T) {
	root := copyGoTestProject(t)
	idx, _ := Open(root)
	_ = idx.EnsureReady(context.Background())

	got := BuildFailureContext(idx, []string{"", "  "}, "go test ./...", strings.Repeat("x", 300))
	if !strings.Contains(got, "no mapped packages") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "...") {
		t.Fatal("expected stderr truncation")
	}
}

func TestBuildFailureContextDuplicateSources(t *testing.T) {
	root := copyGoTestProject(t)
	idx, _ := Open(root)
	_ = idx.EnsureReady(context.Background())
	got := BuildFailureContext(idx, []string{
		"internal/gamma/gamma.go",
		"internal/gamma/gamma.go",
	}, "go test ./...", "fail")
	if got == "" {
		t.Fatal("expected context")
	}
}

func TestLoadIndexLegacyWithoutEdges(t *testing.T) {
	dir := t.TempDir()
	id := string(NewGoID("example.com/x"))
	payload := indexSnapshot{
		Version: IndexVersion,
		Root:    dir,
		BuiltAt: time.Now().UTC(),
		Nodes: map[string]*Node{
			id: {ID: NodeID(id), Kind: KindInternalGo, Name: "example.com/x"},
		},
		Out: map[string][]string{},
		In:  map[string][]string{},
		Stats: Stats{NodeCount: 1},
	}
	b, _ := json.Marshal(payload)
	if err := os.WriteFile(filepath.Join(dir, indexFileName), b, 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex legacy: %v", err)
	}
	if len(g.Nodes) != 1 {
		t.Fatalf("nodes = %d", len(g.Nodes))
	}
}

func TestLoadIndexCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, indexFileName), []byte("{bad"), 0o644)
	_, err := LoadIndex(dir)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseNodeIDInvalid(t *testing.T) {
	if _, err := ParseNodeID("not-a-valid-id"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenCorruptIndex(t *testing.T) {
	root := copyGoTestProject(t)
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, indexFileName), []byte("[]"), 0o644)
	if _, err := Open(root); err == nil {
		t.Fatal("expected open error")
	}
}

func TestIndexEnsureReadyTwice(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestBuildGraphEmptyRoot(t *testing.T) {
	dir := t.TempDir()
	g, meta, err := BuildGraph(BuildOptions{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	if g.Stats.NodeCount != 0 {
		t.Fatalf("nodes = %d", g.Stats.NodeCount)
	}
	if meta == nil {
		t.Fatal("expected meta")
	}
}

func TestRefreshNilIndex(t *testing.T) {
	var idx *Index
	if err := idx.RefreshIfStale(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestCrossRealmAnalyzerErrorIgnored(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("x")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "x", Meta: NodeMeta{BridgeMethod: "Submit"}})
	g.Impact[id] = ImpactLayers{}
	res, err := AffectedByWithAnalyzer(g, id, mockBridgeAnalyzer{ok: true, err: errors.New("fail")})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CrossRealm) != 0 {
		t.Fatalf("CrossRealm = %+v", res.CrossRealm)
	}
}

func TestIndexImportsOfMissing(t *testing.T) {
	root := copyGoTestProject(t)
	idx, _ := Open(root)
	_ = idx.EnsureReady(context.Background())
	if _, err := idx.ImportsOf(NewGoID("missing")); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestIndexAffectedByPath(t *testing.T) {
	root := copyGoTestProject(t)
	idx, _ := Open(root)
	_ = idx.EnsureReady(context.Background())
	res, err := idx.AffectedByPath("internal/gamma")
	if err != nil {
		t.Fatal(err)
	}
	if res.Source == "" {
		t.Fatal("expected source")
	}
}

func TestGraphAddNodeNil(t *testing.T) {
	var g *Graph
	g.AddNode(&Node{ID: NewGoID("x"), Kind: KindInternalGo, Name: "x"})
}

func TestRefreshBlocksDuringRebuild(t *testing.T) {
	root := copyGoTestProject(t)
	idx, _ := Open(root)
	done := make(chan struct{})
	go func() {
		_ = idx.RefreshIfStale(context.Background())
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	_, _ = idx.Status()
	<-done
}

func TestConcurrentInvalidateAndStatus(t *testing.T) {
	root := copyGoTestProject(t)
	idx, _ := Open(root)
	_ = idx.EnsureReady(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = idx.InvalidateFiles([]string{"x.go"})
			_, _ = idx.Status()
		}()
	}
	wg.Wait()
}

func TestModuleCatalogNilIndex(t *testing.T) {
	var idx *Index
	if cat := idx.ModuleCatalog(); cat != nil {
		t.Fatal("expected nil catalog for nil index")
	}
}

func TestCatalogAdapterNilReceiver(t *testing.T) {
	var a *catalogAdapter
	if _, ok := a.ResolveFile("x"); ok {
		t.Fatal("expected false for nil adapter")
	}
	if _, ok := a.ModuleKind("x"); ok {
		t.Fatal("expected false for nil adapter")
	}
	if err := a.EnsureReady(context.Background()); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("EnsureReady err = %v", err)
	}
	n, e, m := a.Status()
	if n != 0 || e != 0 || m != "" {
		t.Fatalf("Status = (%d, %d, %q)", n, e, m)
	}
}

func TestModuleKindFromNodeAllKinds(t *testing.T) {
	cases := []struct {
		kind Kind
		want string
	}{
		{KindInternalGo, "go"},
		{KindInternalJS, "js"},
		{KindExternalGo, "gomod"},
		{KindStdlib, "gomod"},
		{KindExternalNPM, "npm"},
		{KindWorkspaceNPM, "npm"},
		{KindBridge, "bridge"},
	}
	for _, tc := range cases {
		got, ok := moduleKindFromNode(&Node{Kind: tc.kind})
		if !ok || got != tc.want {
			t.Fatalf("moduleKindFromNode(%q) = (%q, %v)", tc.kind, got, ok)
		}
	}
	if _, ok := moduleKindFromNode(nil); ok {
		t.Fatal("expected false for nil node")
	}
	if _, ok := moduleKindFromNode(&Node{Kind: Kind("unknown")}); ok {
		t.Fatal("expected false for unknown kind")
	}
}

func TestModuleCatalogResolveFileMissing(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	cat := idx.ModuleCatalog()
	_ = cat.EnsureReady(context.Background())
	if _, ok := cat.ResolveFile("no/such/file.go"); ok {
		t.Fatal("expected ResolveFile miss")
	}
	extID := string(NewGoModID("github.com/example/extpkg"))
	kind, ok := cat.ModuleKind(extID)
	if !ok || kind != "gomod" {
		t.Fatalf("ModuleKind external = (%q, %v)", kind, ok)
	}
}

func TestGoBuilderHelperFunctions(t *testing.T) {
	root := t.TempDir()
	pkgDir := filepath.Join(root, "internal", "alpha")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := fileList("a.go"); len(got) != 1 || got[0] != "a.go" {
		t.Fatalf("fileList = %v", got)
	}
	if fileList("") != nil {
		t.Fatal("fileList empty should be nil")
	}
	if relPath(root, "") != "" {
		t.Fatal("relPath empty path")
	}
	if relPath(root, root) != "." {
		t.Fatalf("relPath self = %q", relPath(root, root))
	}

	list := appendUniqueString([]string{"a"}, "a")
	if len(list) != 1 {
		t.Fatalf("appendUniqueString dup = %v", list)
	}
	list = appendUniqueString(list, "b")
	if len(list) != 2 {
		t.Fatalf("appendUniqueString add = %v", list)
	}

	b := &GoBuilder{Timeout: 2 * time.Second}
	if b.timeout() != 2*time.Second {
		t.Fatalf("timeout = %v", b.timeout())
	}
	if (&GoBuilder{}).timeout() != defaultGoListTimeout {
		t.Fatal("expected default timeout")
	}

	file, line := splitGoListPos("internal/alpha/broken.go:12:4")
	if file != "internal/alpha/broken.go" || line != 12 {
		t.Fatalf("splitGoListPos = (%q, %d)", file, line)
	}
	file, line = splitGoListPos("simple.go")
	if file != "simple.go" || line != 0 {
		t.Fatalf("splitGoListPos short = (%q, %d)", file, line)
	}
	file, line = splitGoListPos("C:/src/main.go:7:2")
	if file != "C:/src/main.go" || line != 7 {
		t.Fatalf("splitGoListPos drive = (%q, %d)", file, line)
	}

	if _, err := parseIntDecimal(""); err == nil {
		t.Fatal("expected error for empty parseIntDecimal")
	}
	if _, err := parseIntDecimal("12x"); err == nil {
		t.Fatal("expected error for non-digit parseIntDecimal")
	}
	if n, err := parseIntDecimal("42"); err != nil || n != 42 {
		t.Fatalf("parseIntDecimal(42) = (%d, %v)", n, err)
	}

	absFile := filepath.Join(root, "internal", "x.go")
	pe := parseErrorFromGoList(root, pkgDir, goListError{Pos: absFile + ":5:1", Err: "syntax"})
	if pe.File != "internal/x.go" || pe.Line != 5 {
		t.Fatalf("parseErrorFromGoList abs = %+v", pe)
	}
	pe = parseErrorFromGoList(root, pkgDir, goListError{Pos: "internal/rel.go:3:2", Err: "err"})
	if pe.File != "internal/rel.go" || pe.Line != 3 {
		t.Fatalf("parseErrorFromGoList rel path = %+v", pe)
	}
	pe = parseErrorFromGoList(root, pkgDir, goListError{Pos: "local.go:2:1", Err: "err"})
	if pe.File != "internal/alpha/local.go" {
		t.Fatalf("parseErrorFromGoList local = %+v", pe)
	}
	pe = parseErrorFromGoList(root, pkgDir, goListError{Err: "pkg error"})
	if pe.File != "internal/alpha" {
		t.Fatalf("parseErrorFromGoList pkgDir = %+v", pe)
	}
	if pe2 := parseErrorFromGoList(root, pkgDir, goListError{Err: "  "}); pe2.Message != "" {
		t.Fatalf("parseErrorFromGoList empty err = %+v", pe2)
	}
}

func TestDecodeGoListJSONEmpty(t *testing.T) {
	if _, err := decodeGoListJSON(nil); err == nil {
		t.Fatal("expected error for empty go list output")
	}
}

func TestMergeBuildResultMergesExistingNode(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/a")
	to := NewGoID("example.com/b")
	g.AddNode(&Node{ID: id, Name: "old", Files: []string{"a.go"}})
	g.Files["a.go"] = id

	mergeBuildResult(g, []*Node{{
		ID:    id,
		Name:  "new",
		Dir:   "pkg/a",
		Files: []string{"b.go", "a.go"},
		Meta: NodeMeta{
			Version:     "v1",
			BuildMethod: "test",
			Warnings:    []string{"warn1", "warn1"},
		},
	}}, []Edge{{From: id, To: to, Kind: EdgeSourceImport, Files: []string{"b.go"}}}, nil)

	n := g.Nodes[id]
	if n.Name != "new" || n.Dir != "pkg/a" {
		t.Fatalf("merged node = %+v", n)
	}
	if len(n.Files) != 2 || n.Meta.Version != "v1" {
		t.Fatalf("merged files/meta = %+v", n)
	}
	if len(n.Meta.Warnings) != 1 {
		t.Fatalf("warnings = %v", n.Meta.Warnings)
	}
	if g.Files["b.go"] != id {
		t.Fatal("expected file mapping for b.go")
	}
	if len(g.Out[id]) != 1 || g.Out[id][0] != to {
		t.Fatalf("Out = %v", g.Out[id])
	}

	var nilG *Graph
	mergeBuildResult(nilG, nil, nil, nil)
	mergeBuildResult(g, []*Node{nil}, nil, nil)
}

func TestGraphImportsOfAndNil(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("a")
	b := NewGoID("b")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "b"})
	g.AddEdge(a, b, EdgeSourceImport, "a.go")

	imports := g.ImportsOf(a)
	if len(imports) != 1 || imports[0] != b {
		t.Fatalf("ImportsOf = %v", imports)
	}
	if importers := g.ImportedBy(b); len(importers) != 1 || importers[0] != a {
		t.Fatalf("ImportedBy = %v", importers)
	}

	var nilG *Graph
	if nilG.ImportsOf(a) != nil {
		t.Fatal("nil graph ImportsOf should be nil")
	}
	if nilG.ImportedBy(a) != nil {
		t.Fatal("nil graph ImportedBy should be nil")
	}
	if _, ok := nilG.Node(a); ok {
		t.Fatal("nil graph Node should be false")
	}
}

func TestIndexBridgeAnalyzerCrossRealm(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/desktop")
	g.AddNode(&Node{
		ID:   id,
		Kind: KindInternalGo,
		Name: "desktop",
		Meta: NodeMeta{BridgeMethod: "Submit"},
	})
	g.Impact[id] = ImpactLayers{}

	idx := &Index{root: "/tmp", graph: g}
	idx.SetBridgeImpactAnalyzer(mockBridgeAnalyzer{
		ok: true,
		entries: []CrossRealmImpactEntry{
			{ID: "ui:Panel", Name: "Panel", Kind: "ui_component"},
		},
	})

	res, err := idx.AffectedBy(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CrossRealm) != 1 || res.CrossRealm[0].Name != "Panel" {
		t.Fatalf("CrossRealm = %+v", res.CrossRealm)
	}
}

func TestIndexVersionConflictsPopulated(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module root.test\n\ngo 1.21\n\nrequire github.com/foo/bar v1.0.0\n")
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "go.mod"), "module sub.test\n\ngo 1.21\n\nrequire github.com/foo/bar v2.0.0\n")

	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	conflicts, err := idx.VersionConflicts()
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) == 0 {
		t.Fatal("expected version conflicts")
	}
}

func TestIndexImportedByMissing(t *testing.T) {
	root := copyGoTestProject(t)
	idx, _ := Open(root)
	_ = idx.EnsureReady(context.Background())
	if _, err := idx.ImportedBy(NewGoID("missing")); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestFormatImpactNamesAtMaxShowsEllipsis(t *testing.T) {
	entries := make([]ImpactEntry, 5)
	for i := range entries {
		entries[i] = ImpactEntry{Name: fmt.Sprintf("mod%d", i)}
	}
	got := formatImpactNames(entries, 5)
	if !strings.Contains(got, "…") {
		t.Fatalf("expected ellipsis at max, got %q", got)
	}
}

func TestBuildFailureContextManyDirectImporters(t *testing.T) {
	g := NewGraph("/tmp")
	target := NewGoID("example.com/target")
	g.AddNode(&Node{ID: target, Kind: KindInternalGo, Name: "target", Dir: "target"})
	g.Files["target/pkg.go"] = target
	for i := 0; i < 6; i++ {
		id := NewGoID(fmt.Sprintf("example.com/importer%d", i))
		g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: fmt.Sprintf("importer%d", i)})
		g.AddEdge(id, target, EdgeSourceImport, "f.go")
	}
	computeImpact(g)
	g.Impact[target] = computeImpactFor(g, target)

	idx := &Index{root: "/tmp", graph: g}
	got := BuildFailureContext(idx, []string{"target/pkg.go"}, "go test ./...", "fail")
	if !strings.Contains(got, "directly imported by") {
		t.Fatalf("missing direct importers line: %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("expected truncated direct names: %q", got)
	}
}

func TestDependencyToolsMetadataAndEdgeCases(t *testing.T) {
	idx := readyTestIndex(t)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	RegisterTools(nil, idx)
	RegisterTools(reg, nil)

	for _, name := range []string{"dependency_status", "dependency_affected_by", "dependency_imports", "dependency_cycles"} {
		toolDef, ok := reg.Get(name)
		if !ok {
			t.Fatalf("missing %q", name)
		}
		if toolDef.Description() == "" {
			t.Fatalf("%q Description empty", name)
		}
		if !toolDef.ReadOnly() {
			t.Fatalf("%q should be read-only", name)
		}
	}

	statusTool, _ := reg.Get("dependency_status")
	var nilIdx depStatusTool
	if out, err := nilIdx.Execute(context.Background(), nil); err != nil || out != "dependency index unavailable" {
		t.Fatalf("nil idx status = (%q, %v)", out, err)
	}
	if _, err := statusTool.Execute(context.Background(), json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}

	affectedTool, _ := reg.Get("dependency_affected_by")
	if _, err := affectedTool.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected path required error")
	}
	if _, err := affectedTool.Execute(context.Background(), json.RawMessage(`{bad`)); err == nil {
		t.Fatal("expected invalid args error")
	}

	layers := ImpactLayersView{
		Direct:     make([]ImpactEntry, 25),
		Transitive: make([]ImpactEntry, 35),
	}
	for i := range layers.Direct {
		layers.Direct[i] = ImpactEntry{Name: fmt.Sprintf("d%d", i)}
	}
	for i := range layers.Transitive {
		layers.Transitive[i] = ImpactEntry{Name: fmt.Sprintf("t%d", i)}
	}
	truncated := truncateImpactResult(ImpactResult{Layers: layers})
	if len(truncated.Layers.Direct) != 20 || len(truncated.Layers.Transitive) != 30 {
		t.Fatalf("truncate = direct %d transitive %d", len(truncated.Layers.Direct), len(truncated.Layers.Transitive))
	}

	importsTool, _ := reg.Get("dependency_imports")
	gammaPath := "internal/gamma"
	out, err := importsTool.Execute(context.Background(), json.RawMessage(`{"path":"`+gammaPath+`","direction":"imported_by"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "imported_by") {
		t.Fatalf("output = %q", out)
	}
	if _, err := importsTool.Execute(context.Background(), json.RawMessage(`{"path":"`+gammaPath+`","direction":"bad"}`)); err == nil {
		t.Fatal("expected unknown direction error")
	}

	cyclesTool, _ := reg.Get("dependency_cycles")
	cycles := []Cycle{{Lang: "go", Ring: []NodeID{NewGoID("a")}}, {Lang: "js", Ring: []NodeID{NewJSID("x")}}}
	if got := filterCycles(cycles, "go"); len(got) != 1 || got[0].Lang != "go" {
		t.Fatalf("filterCycles go = %+v", got)
	}
	if got := filterCycles(cycles, ""); len(got) != 2 {
		t.Fatalf("filterCycles all = %d", len(got))
	}
	if _, err := cyclesTool.Execute(context.Background(), json.RawMessage(`{"lang":"go"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestBuildGraphMergedGoAndJS(t *testing.T) {
	root := copyGoTestProject(t)
	writeFile(t, filepath.Join(root, "package.json"), `{"name":"merged-root","dependencies":{"lodash":"^4.0.0"}}`)
	g, meta, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if meta == nil || g.Stats.NodeCount == 0 {
		t.Fatal("expected merged graph")
	}
	if g.BuildMethod != BuildMerged {
		t.Fatalf("BuildMethod = %q, want merged", g.BuildMethod)
	}
	if g.Stats.GoPackages == 0 || g.Stats.JSPackages == 0 {
		t.Fatalf("stats = %+v", g.Stats)
	}
}

func TestStoreSaveLoadValidation(t *testing.T) {
	dir := t.TempDir()
	if err := SaveIndex(nil, dir); err == nil {
		t.Fatal("expected error for nil graph")
	}
	if err := SaveIndex(NewGraph(dir), ""); err == nil {
		t.Fatal("expected error for empty dir")
	}
	if err := SaveMeta(nil, dir); err == nil {
		t.Fatal("expected error for nil meta")
	}
	if err := SaveMeta(&Meta{}, ""); err == nil {
		t.Fatal("expected error for empty meta dir")
	}
	if _, err := LoadMeta(""); err == nil {
		t.Fatal("expected error for empty LoadMeta dir")
	}
	if _, err := ProjectDir(""); err == nil {
		t.Fatal("expected error for empty ProjectDir")
	}
}

func TestSaveLoadWithCyclesAndConflicts(t *testing.T) {
	dir := t.TempDir()
	g := NewGraph(dir)
	g.AddNode(&Node{ID: NewGoID("a"), Kind: KindInternalGo, Name: "a"})
	g.Cycles = []Cycle{{Lang: "go", Ring: []NodeID{NewGoID("a"), NewGoID("b")}}}
	g.Conflicts = []VersionConflict{{
		Module:   "github.com/foo/bar",
		Versions: []string{"v1.0.0", "v2.0.0"},
		Paths:    []string{"go.mod", "sub/go.mod"},
	}}
	if err := SaveIndex(g, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Cycles) != 1 || len(loaded.Conflicts) != 1 {
		t.Fatalf("cycles=%d conflicts=%d", len(loaded.Cycles), len(loaded.Conflicts))
	}
}

func TestNodeLangRealmFallback(t *testing.T) {
	if got := nodeLang(NewGoID("bridge:Submit"), &Node{Kind: KindBridge, Name: "Submit"}); got != "go" {
		t.Fatalf("bridge nodeLang = %q", got)
	}
	if got := nodeLang(NewJSID("frontend/lib"), &Node{Kind: KindBridge}); got != "js" {
		t.Fatalf("js bridge nodeLang = %q", got)
	}
}

func TestFindOrphansAndSampleParseErrors(t *testing.T) {
	g := NewGraph("/tmp")
	ext := NewGoModID("github.com/x")
	leaf := NewGoID("example.com/leaf")
	g.AddNode(&Node{ID: ext, Kind: KindExternalGo, Name: "github.com/x"})
	g.AddNode(&Node{ID: leaf, Kind: KindInternalGo, Name: "leaf"})
	orphans := findOrphans(g)
	if len(orphans) != 1 || orphans[0] != leaf {
		t.Fatalf("orphans = %v", orphans)
	}

	errs := make([]ParseError, 15)
	for i := range errs {
		errs[i] = ParseError{File: fmt.Sprintf("f%d.go", i), Message: "err"}
	}
	if got := sampleParseErrors(errs, 10); len(got) != 10 {
		t.Fatalf("sample = %d", len(got))
	}
	if got := sampleParseErrors(nil, 10); got != nil {
		t.Fatal("expected nil sample for empty input")
	}
}

func TestBuildNodeSectionGoOnly(t *testing.T) {
	root := copyGoTestProject(t)
	nodes, edges, errs, err := buildNodeSection(root)
	if err == nil {
		t.Fatal("expected error without package.json")
	}
	if nodes != nil || edges != nil || errs != nil {
		t.Fatalf("expected nil results on error: nodes=%v edges=%v errs=%v", nodes, edges, errs)
	}
}

func TestGoListPackageErrorsSkipsEmpty(t *testing.T) {
	if got := goListPackageErrors("/tmp", goListPackage{
		DepsErrors: []goListError{{Err: "  "}},
	}); len(got) != 0 {
		t.Fatalf("got = %+v", got)
	}
}

func TestFirstGoFileInDir(t *testing.T) {
	root := copyGoTestProject(t)
	dir := filepath.Join(root, "internal", "alpha")
	if firstGoFileInDir(dir) == "" {
		t.Fatal("expected go file in alpha dir")
	}
	if firstGoFileInDir(t.TempDir()) != "" {
		t.Fatal("expected empty for missing dir")
	}
}

func TestGraphAddEdgeDuplicateAndRemove(t *testing.T) {
	g := NewGraph("/tmp")
	a, b, c := NewGoID("a"), NewGoID("b"), NewGoID("c")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "b"})
	g.AddNode(&Node{ID: c, Kind: KindInternalGo, Name: "c"})
	g.AddEdge(a, b, EdgeSourceImport, "a.go")
	g.AddEdge(a, b, EdgeSourceImport, "a.go")
	g.AddEdge(a, b, EdgeSourceImport, "b.go")
	if len(g.edges) != 1 || len(g.edges[0].Files) != 2 {
		t.Fatalf("edges = %+v", g.edges)
	}
	g.Files["a.go"] = a
	g.Orphans = []NodeID{a}
	g.RemoveNode(a)
	if _, ok := g.Nodes[a]; ok {
		t.Fatal("node should be removed")
	}
	if g.Files["a.go"] != "" {
		t.Fatal("file mapping should be removed")
	}
}

func TestSourceImportAdjacencyHelpers(t *testing.T) {
	g := NewGraph("/tmp")
	a, b, ext := NewGoID("a"), NewGoID("b"), NewGoModID("github.com/x")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "b"})
	g.AddNode(&Node{ID: ext, Kind: KindExternalGo, Name: "github.com/x"})
	g.AddEdge(a, b, EdgeSourceImport, "a.go")
	g.AddEdge(a, ext, EdgeManifestRequire, "go.mod")

	if importers := sourceImportImporters(g, b); len(importers) != 1 || importers[0] != a {
		t.Fatalf("importers = %v", importers)
	}
	if imports := sourceImportImports(g, a); len(imports) != 1 || imports[0] != b {
		t.Fatalf("imports = %v", imports)
	}
	if sourceImportImporters(nil, b) != nil {
		t.Fatal("nil graph importers")
	}
}

func TestClassifyImportPaths(t *testing.T) {
	if id, kind := classifyImport("", "", "", true); id != "" || kind != "" {
		t.Fatalf("empty import = (%q, %q)", id, kind)
	}
	id, kind := classifyImport("example.com/mod", "fmt", "", false)
	if kind != KindStdlib || id != NewStdlibID("fmt") {
		t.Fatalf("fmt heuristic = (%q, %q)", id, kind)
	}
	id, kind = classifyImport("example.com/mod", "example.com/mod/internal/pkg", "", true)
	if kind != KindStdlib {
		t.Fatalf("fromGoListStandard = (%q, %q)", id, kind)
	}
	id, kind = classifyImport("example.com/mod", "example.com/mod/internal/pkg", "", false)
	if kind != KindInternalGo || id != NewGoID("example.com/mod/internal/pkg") {
		t.Fatalf("internal = (%q, %q)", id, kind)
	}
	if mod := modulePathForImport("github.com/foo/bar/v2/baz"); mod != "github.com/foo/bar/v2" {
		t.Fatalf("modulePathForImport = %q", mod)
	}
	if mod := modulePathForImport("strings"); mod != "strings" {
		t.Fatalf("modulePathForImport std = %q", mod)
	}
}

func TestStdlibExistsUnderGOROOT(t *testing.T) {
	goroot := t.TempDir()
	src := filepath.Join(goroot, "src", "mypkg")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "mypkg.go"), []byte("package mypkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !stdlibExistsUnderGOROOT(goroot, "mypkg") {
		t.Fatal("expected stdlib dir with go file")
	}
	if stdlibExistsUnderGOROOT(goroot, "missing") {
		t.Fatal("expected false for missing import")
	}
	emptyDir := filepath.Join(goroot, "src", "empty")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if stdlibExistsUnderGOROOT(goroot, "empty") {
		t.Fatal("expected false for empty dir")
	}
}

func TestParseGoModReplaceDirectives(t *testing.T) {
	modPath := filepath.Join(t.TempDir(), "go.mod")
	body := "module example.com/x\n\ngo 1.21\n\nreplace github.com/old => github.com/new\n\nreplace (\n  github.com/a => github.com/b\n)\n"
	if err := os.WriteFile(modPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := parseGoMod(modPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Replaces) != 2 {
		t.Fatalf("replaces = %+v", info.Replaces)
	}
}

func TestNewGoBuilderValidation(t *testing.T) {
	if _, err := NewGoBuilder(""); err == nil {
		t.Fatal("expected error for empty root")
	}
}

func TestGoBuilderReplaceWarnings(t *testing.T) {
	root := copyGoTestProject(t)
	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	nodes, _, _, _, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	rootID := NewGoID("example.com/testproj")
	for _, n := range nodes {
		if n.ID == rootID && len(n.Meta.Warnings) == 0 {
			t.Fatalf("expected replace warnings on root node: %+v", n.Meta)
		}
	}
}

func TestComputeImpactSkipsExternal(t *testing.T) {
	g := NewGraph("/tmp")
	ext := NewGoModID("github.com/x")
	g.AddNode(&Node{ID: ext, Kind: KindExternalGo, Name: "github.com/x"})
	computeImpact(g)
	if _, ok := g.Impact[ext]; ok {
		t.Fatal("external node should not have impact")
	}
}

func TestCatalogModuleKindMissing(t *testing.T) {
	root := copyGoTestProject(t)
	idx, _ := Open(root)
	cat := idx.ModuleCatalog()
	_ = cat.EnsureReady(context.Background())
	if _, ok := cat.ModuleKind("missing:module"); ok {
		t.Fatal("expected missing module kind")
	}
}

func TestIndexResolveIDByPrefix(t *testing.T) {
	root := copyGoTestProject(t)
	idx, _ := Open(root)
	_ = idx.EnsureReady(context.Background())
	id, err := idx.ResolveID("internal/gamma/gamma.go")
	if err != nil {
		t.Fatal(err)
	}
	if id != NewGoID("example.com/testproj/internal/gamma") {
		t.Fatalf("ResolveID = %q", id)
	}
}

func TestParseNodeIDValidation(t *testing.T) {
	if _, err := ParseNodeID("bad:realm:extra"); err == nil {
		t.Fatal("expected invalid realm error")
	}
	if _, err := ParseNodeID("go:pkg#sym"); err == nil {
		t.Fatal("expected symbol error")
	}
	if _, err := ParseNodeID("go:pkg:10#sym"); err == nil {
		t.Fatal("expected line+symbol error")
	}
	var bad NodeID = "not-valid"
	if bad.Realm() != "" {
		t.Fatal("invalid id Realm should be empty")
	}
	if bad.Path() != "not-valid" {
		t.Fatalf("invalid id Path = %q", bad.Path())
	}
}

func TestCycleHelperFunctions(t *testing.T) {
	if computeCycles(nil) != nil {
		t.Fatal("nil graph cycles")
	}
	if hasSourceImportEdge(nil, NewGoID("a"), NewGoID("b")) {
		t.Fatal("nil graph edge")
	}
	g := NewGraph("/tmp")
	a, b := NewGoID("a"), NewGoID("b")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "alpha"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "beta"})
	g.AddEdge(a, b, EdgeSourceImport, "a.go")
	if !hasSourceImportEdge(g, a, b) {
		t.Fatal("expected source import edge")
	}
	if got := hubDisplayName(g, a); got != "alpha" {
		t.Fatalf("hubDisplayName = %q", got)
	}
	if got := hubDisplayName(nil, NewGoID("x")); got != "go:x" {
		t.Fatalf("hubDisplayName fallback = %q", got)
	}
}

func TestDiscoverableEmptyRoot(t *testing.T) {
	if Discoverable("") {
		t.Fatal("empty root should not be discoverable")
	}
}

func TestStdlibExistsUnderGOROOTSingleFile(t *testing.T) {
	goroot := t.TempDir()
	filePath := filepath.Join(goroot, "src", "singlefile.go")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte("package singlefile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !stdlibExistsUnderGOROOT(goroot, "singlefile.go") {
		t.Fatal("expected true for single .go file under GOROOT/src")
	}
}

func TestParseReplaceLineInvalid(t *testing.T) {
	if rep := parseReplaceLine("no arrow here"); rep.OldPath != "" {
		t.Fatalf("parseReplaceLine invalid = %+v", rep)
	}
}

func TestGoBuilderEnsureNodeEmptyID(t *testing.T) {
	b := &GoBuilder{ModulePath: "example.com/x"}
	m := map[NodeID]*Node{}
	b.ensureNode(m, "", KindInternalGo, "x", BuildGoList)
	if len(m) != 0 {
		t.Fatal("empty id should not add node")
	}
}

func TestCatalogStatusBeforeReady(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	cat := idx.ModuleCatalog()
	n, e, m := cat.Status()
	if n != 0 || e != 0 || m != "" {
		t.Fatalf("Status before ready = (%d, %d, %q)", n, e, m)
	}
}

func TestOpenEmptyRoot(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("expected error for empty root")
	}
}

func TestLoadMetaCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, metaFileName), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMeta(dir); err == nil {
		t.Fatal("expected corrupt meta error")
	}
}

func TestDependencyAffectedByToolTruncate(t *testing.T) {
	g := NewGraph("/tmp")
	source := NewGoID("example.com/source")
	g.AddNode(&Node{ID: source, Kind: KindInternalGo, Name: "source"})
	for i := 0; i < 25; i++ {
		id := NewGoID(fmt.Sprintf("example.com/imp%d", i))
		g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: fmt.Sprintf("imp%d", i)})
		g.AddEdge(id, source, EdgeSourceImport, "f.go")
	}
	computeImpact(g)
	g.Files["source/f.go"] = source
	idx := &Index{root: "/tmp", graph: g}

	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	affectedTool, _ := reg.Get("dependency_affected_by")
	out, err := affectedTool.Execute(context.Background(), json.RawMessage(`{"path":"source/f.go"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Impact for") {
		t.Fatalf("output = %q", out)
	}
}
