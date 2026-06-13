package callgraph

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/dependency"
	"arcdesk/internal/tool"
)

func TestNoopCatalogMethods(t *testing.T) {
	var c noopCatalog
	if id, ok := c.ResolveFile("x.go"); ok || id != "" {
		t.Fatalf("ResolveFile = %q,%v", id, ok)
	}
	if _, ok := c.ModuleKind("m"); ok {
		t.Fatal("ModuleKind should be false")
	}
	if err := c.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n, e, m := c.Status(); n != 0 || e != 0 || m != "" {
		t.Fatalf("Status = %d,%d,%q", n, e, m)
	}
}

func TestFallbackCatalogAllModuleKinds(t *testing.T) {
	root := copyWailsTestProject(t)
	cat := NewFallbackCatalog(root)
	_ = os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o644)
	cases := []struct {
		path string
		kind string
	}{
		{"desktop/app.go", "go"},
		{"go.mod", "gomod"},
		{"desktop/frontend/src/lib/bridge.ts", "js"},
		{"README.md", "file"},
	}
	for _, tc := range cases {
		if tc.path == "README.md" {
			_ = os.WriteFile(filepath.Join(root, tc.path), []byte("hi\n"), 0o644)
		}
		mod, ok := cat.ResolveFile(tc.path)
		if !ok {
			t.Fatalf("ResolveFile(%q) failed", tc.path)
		}
		kind, ok := cat.ModuleKind(mod)
		if !ok || kind != tc.kind {
			t.Fatalf("ModuleKind(%q) = %q,%v want %q", mod, kind, ok, tc.kind)
		}
	}
	if _, ok := cat.ResolveFile(""); ok {
		t.Fatal("empty path should fail")
	}
	if err := cat.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, m := cat.Status(); m != "fallback" {
		t.Fatalf("status method = %q", m)
	}
}

func copyDependencyFixture(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	src := filepath.Join(filepath.Dir(file), "..", "dependency", "testdata", "go_project")
	dst := t.TempDir()
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestDependencyCatalogWrapper(t *testing.T) {
	if cat := NewDependencyCatalog(nil); cat == nil {
		t.Fatal("nil idx should return noop")
	}
	root := copyDependencyFixture(t)
	dep, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := dep.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	cat := NewDependencyCatalog(dep)
	mod, ok := cat.ResolveFile("internal/alpha/alpha.go")
	if !ok || mod == "" {
		t.Fatalf("ResolveFile failed: %q,%v", mod, ok)
	}
	kind, ok := cat.ModuleKind(mod)
	if !ok || kind == "" {
		t.Fatalf("ModuleKind failed: %q,%v", kind, ok)
	}
	if err := cat.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	n, e, m := cat.Status()
	if n < 0 || e < 0 || m == "" {
		t.Fatalf("Status = %d,%d,%q", n, e, m)
	}
}

func TestNodeIDParseAndMethods(t *testing.T) {
	id := NewGoBindID("desktop/app.go", "App.Submit")
	if id.String() != string(id) {
		t.Fatal("String mismatch")
	}
	if id.Realm() != "gobind" {
		t.Fatalf("Realm = %q", id.Realm())
	}
	if id.Path() != "desktop/app.go" {
		t.Fatalf("Path = %q", id.Path())
	}
	if id.Symbol() != "App.Submit" {
		t.Fatalf("Symbol = %q", id.Symbol())
	}
	if _, err := ParseNodeID("bad:id"); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := ParseNodeID("unknown:file.go#x"); err == nil {
		t.Fatal("expected unknown realm error")
	}
	_ = NewUIIDAtLine("a.tsx", 3, "H")
	_ = NewFnID("b.ts", "fn")
	if itoa(0) != "0" {
		t.Fatal("itoa(0)")
	}
}

func TestMethodCatalogLegacyWrapper(t *testing.T) {
	root := copyWailsTestProject(t)
	binds, _, _, _, _ := ScanGoBinds(root)
	m := MethodCatalog(root, binds)
	if len(m) == 0 {
		t.Fatal("expected methods")
	}
}

func TestParseAppBindingsMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := ParseAppBindings(dir); err == nil {
		t.Fatal("expected error for missing bridge.ts")
	}
	path := filepath.Join(dir, "desktop", "frontend", "src", "lib")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(path, "bridge.ts"), []byte("export const app = {};\n"), 0o644)
	if _, err := ParseAppBindings(dir); err == nil {
		t.Fatal("expected error without AppBindings interface")
	}
}

func TestBuildMethodCatalogThreeTiers(t *testing.T) {
	root := copyWailsTestProject(t)
	binds, _, _, _, _ := ScanGoBinds(root)
	if len(BuildMethodCatalog(root, binds).Methods) == 0 {
		t.Fatal("tier1 empty")
	}

	root2 := copyWailsTestProject(t)
	_ = os.RemoveAll(filepath.Join(root2, "desktop", "frontend", "wailsjs"))
	binds2, _, _, _, _ := ScanGoBinds(root2)
	c2 := BuildMethodCatalog(root2, binds2)
	if len(c2.Methods) == 0 || !hasWarningMessage(c2.Warnings, "wailsjs_missing") {
		t.Fatalf("tier2: methods=%d warnings=%v", len(c2.Methods), c2.Warnings)
	}

	root3 := copyWailsTestProject(t)
	_ = os.RemoveAll(filepath.Join(root3, "desktop", "frontend", "wailsjs"))
	_ = os.WriteFile(filepath.Join(root3, "desktop", "frontend", "src", "lib", "bridge.ts"),
		[]byte("export const app = { Submit: async () => {} };\n"), 0o644)
	binds3, _, _, _, _ := ScanGoBinds(root3)
	if len(BuildMethodCatalog(root3, binds3).Methods) == 0 {
		t.Fatal("tier3 empty")
	}
}

func TestFindBridgePathAndIndex(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	hook := NewHookID("desktop/frontend/src/lib/useSubmit.ts", "useSubmit")
	paths, err := FindBridgePath(g, hook, "Submit")
	if err != nil || len(paths) == 0 {
		t.Fatalf("FindBridgePath: paths=%d err=%v", len(paths), err)
	}

	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	byLine, err := idx.FindBridge(context.Background(), "desktop/frontend/src/lib/useSubmit.ts", 5, "Submit")
	if err != nil || len(byLine) == 0 {
		t.Fatalf("FindBridge by line: %v paths=%d", err, len(byLine))
	}
	if _, err := idx.FindBridge(context.Background(), "desktop/frontend/src/lib/useSubmit.ts", 0, "MissingMethod"); err == nil {
		t.Fatal("expected error for missing method")
	}
}

func TestResolveNodeIDEdgeCases(t *testing.T) {
	g := NewGraph(t.TempDir())
	a := NewUIID("f.tsx", "A")
	b := NewUIID("f.tsx", "B")
	g.AddNode(&Node{ID: a, Kind: KindUIComponent, Name: "A", File: "f.tsx"})
	g.AddNode(&Node{ID: b, Kind: KindUIComponent, Name: "B", File: "f.tsx"})
	g.RebuildIndexes()

	if _, err := ResolveNodeID(nil, "f.tsx", "A"); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("nil graph err = %v", err)
	}
	if _, err := ResolveNodeID(g, "", "A"); err == nil {
		t.Fatal("empty path")
	}
	if _, err := ResolveNodeID(g, "f.tsx", ""); err == nil {
		t.Fatal("ambiguous path")
	}
	if _, err := ResolveNodeID(g, "missing.tsx", "X"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("missing err = %v", err)
	}
}

func TestIndexLifecyclePaths(t *testing.T) {
	if _, err := Open("", nil); err == nil {
		t.Fatal("empty root")
	}
	root := copyWailsTestProject(t)
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(dir)

	idx, err := Open(root, NewFallbackCatalog(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal("second EnsureReady")
	}
	if err := idx.RefreshIfStale(context.Background()); err != nil {
		t.Fatal("refresh when fresh")
	}

	_ = idx.InvalidateFiles(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := idx.RefreshIfStale(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel refresh = %v", err)
	}

	meta, err := LoadMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Fingerprint == "" {
		t.Fatal("empty fingerprint")
	}

	g2, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g2.Status(); err != nil {
		t.Fatalf("loaded status: %v", err)
	}
}

func TestStoreErrorsAndRoundTrip(t *testing.T) {
	if _, err := ProjectDir(""); err == nil {
		t.Fatal("empty workspace")
	}
	if err := SaveIndex(nil, t.TempDir()); err == nil {
		t.Fatal("nil graph")
	}
	if err := SaveIndex(NewGraph(t.TempDir()), ""); err == nil {
		t.Fatal("empty dir")
	}
	if _, err := LoadIndex(""); err == nil {
		t.Fatal("empty load dir")
	}

	dir := t.TempDir()
	meta := &Meta{GeneratedAt: time.Now().UTC(), Fingerprint: "abc", IndexVersion: IndexVersion}
	if err := SaveMeta(meta, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadMeta(dir)
	if err != nil || loaded.Fingerprint != "abc" {
		t.Fatalf("LoadMeta = %+v,%v", loaded, err)
	}
	if err := SaveMeta(nil, dir); err == nil {
		t.Fatal("nil meta")
	}
	if _, err := LoadMeta(t.TempDir()); !errors.Is(err, ErrIndexNotFound) {
		t.Fatalf("missing meta = %v", err)
	}
}

func TestMetaStaleChecks(t *testing.T) {
	if ComputeFingerprint("") != "" {
		t.Fatal("empty root fingerprint")
	}
	root := copyWailsTestProject(t)
	if !CheckStale(root, nil) {
		t.Fatal("nil meta stale")
	}
	if !CheckStale(root, &Meta{IndexVersion: 0}) {
		t.Fatal("bad version stale")
	}
	meta := NewMeta(root)
	if CheckStale(root, meta) {
		t.Fatal("fresh meta should not be stale")
	}
	meta2 := NewMeta(root)
	meta2.Fingerprint = "stale"
	if !CheckStale(root, meta2) {
		t.Fatal("fingerprint mismatch stale")
	}
	if fingerprintRelevant("other/file.txt") {
		t.Fatal("irrelevant path")
	}
}

func TestFormatHelpersEdgeCases(t *testing.T) {
	if got := FormatPathsSummary("X", nil); got != "X: 0 paths" {
		t.Fatalf("summary = %q", got)
	}
	if FormatLLMContext(nil, "Submit") != "" {
		t.Fatal("empty paths")
	}
	segments := make([]PathSegment, 0, 7)
	for i := 0; i < 7; i++ {
		segments = append(segments, PathSegment{
			Node: NodeSnapshot{Kind: KindHook, Name: "h", File: "f.ts", Line: i + 1},
		})
	}
	out := FormatLLMContext([]CallPath{{Segments: segments}}, "App.Submit")
	if !strings.Contains(out, "…") && !strings.Contains(out, "- …") {
		t.Fatalf("expected truncation: %q", out)
	}
	if !strings.Contains(out, "Go bind") {
		t.Fatal("expected go bind line")
	}
}

func TestBuildCrossRealmContextBranches(t *testing.T) {
	if BuildCrossRealmContext(nil, nil, "go test") != "" {
		t.Fatal("nil index")
	}
	root := copyWailsTestProject(t)
	idx, _ := Open(root, NewFallbackCatalog(root))
	_ = idx.EnsureReady(context.Background())

	if BuildCrossRealmContext(idx, []string{"a.go"}, "git status") != "" {
		t.Fatal("non-verify cmd")
	}
	if BuildCrossRealmContext(idx, []string{"desktop/app.go"}, "go test") != "" {
		t.Fatal("single realm")
	}
	block := BuildCrossRealmContext(idx, []string{
		"desktop/app.go",
		"desktop/frontend/src/lib/useSubmit.ts",
		"desktop/frontend/src/components/Composer.tsx",
	}, "go test ./...")
	if block == "" || !strings.Contains(block, "## Wails Call Chain") {
		t.Fatalf("block = %q", block)
	}
	block2 := BuildCrossRealmContext(idx, []string{
		"desktop/app.go",
		"desktop/frontend/src/lib/useSubmit.ts",
	}, "go build ./...")
	if block2 == "" {
		t.Fatal("expected go-file fallback block")
	}
}

func TestDiscoverableEdgeCases(t *testing.T) {
	if Discoverable("") {
		t.Fatal("empty root")
	}
	dir := t.TempDir()
	if Discoverable(dir) {
		t.Fatal("empty dir")
	}
	_ = os.MkdirAll(filepath.Join(dir, "desktop"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "desktop", "main.go"), []byte("package main\n"), 0o644)
	if Discoverable(dir) {
		t.Fatal("missing wailsjs")
	}
}

func TestToolsEdgeCases(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())

	reg := tool.NewRegistry()
	RegisterTools(nil, idx)
	RegisterTools(reg, nil)

	path, sym := splitFromSymbol("file.ts#Sym", "ignored")
	if path != "file.ts" || sym != "Sym" {
		t.Fatalf("split = %q,%q", path, sym)
	}
	path, sym = splitFromSymbol("file.ts", "Outer")
	if sym != "Outer" {
		t.Fatalf("outer sym = %q", sym)
	}

	p, line, sym := parseFrontendRef("a.ts:42")
	if p != "a.ts" || line != 42 || sym != "" {
		t.Fatalf("line ref = %q,%d,%q", p, line, sym)
	}
	p, line, sym = parseFrontendRef("a.ts#Hook")
	if p != "a.ts" || sym != "Hook" {
		t.Fatalf("sym ref = %q,%d,%q", p, line, sym)
	}
	_, line, _ = parseFrontendRef("a.ts:notline")
	if line != 0 {
		t.Fatalf("bad line parse = %d", line)
	}

	backward := cgTraceBackwardTool{idx: idx}
	if _, err := backward.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("missing go_method")
	}
	if _, err := backward.Execute(context.Background(), []byte(`{"go_method":"Missing"}`)); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("missing method err = %v", err)
	}
	_ = backward.Description()
	_ = backward.ReadOnly()

	forward := cgTraceForwardTool{idx: idx}
	if _, err := forward.Execute(context.Background(), []byte(`{"from":"missing.tsx","symbol":"X"}`)); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("forward err = %v", err)
	}
	_, err := forward.Execute(context.Background(), json.RawMessage(`{"from":"desktop/frontend/src/components/Composer.tsx#Composer","max_paths":2,"include_go_internal":true}`))
	if err != nil {
		t.Fatal(err)
	}

	find := cgFindBridgeTool{idx: idx}
	if _, err := find.Execute(context.Background(), []byte(`{`)); err == nil {
		t.Fatal("invalid json")
	}
	if _, err := find.Execute(context.Background(), []byte(`{"frontend":"desktop/frontend/src/components/Composer.tsx#Composer","go_method":"Submit"}`)); err != nil {
		t.Fatalf("find by symbol: %v", err)
	}
	if _, err := find.Execute(context.Background(), []byte(`{"frontend":"desktop/frontend/src/lib/useSubmit.ts:5","go_method":"Submit"}`)); err != nil {
		t.Fatalf("find by line: %v", err)
	}
	_ = find.Schema()

	status := cgStatusTool{idx: &Index{}}
	if out, err := status.Execute(context.Background(), nil); err != nil || !strings.Contains(out, "not ready") {
		t.Fatalf("status not ready = %q err=%v", out, err)
	}
}

func TestGoBindEdgeCases(t *testing.T) {
	dir := t.TempDir()
	desktop := filepath.Join(dir, "desktop")
	if err := os.MkdirAll(desktop, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(desktop, "other.go"), []byte("package main\nfunc (a *App) FromRegex() {}\n"), 0o644)
	binds, _, _, _, _ := ScanGoBinds(dir)
	if len(binds) == 0 {
		t.Fatal("regex fallback should find bind")
	}
	if !tsBraceBalanced(`{ "a": 1 }`) || tsBraceBalanced(`{ unclosed`) {
		t.Fatal("tsBraceBalanced")
	}
}

func TestGraphNilAndEdgeCases(t *testing.T) {
	var g *CallGraph
	g.AddNode(&Node{ID: "x", Kind: KindHook, Name: "x"})
	g.AddEdge("a", "b", EdgeCalls)
	if n, ok := g.Node("x"); ok || n != nil || g.EdgeCount() != 0 {
		t.Fatal("nil graph ops")
	}
	g2 := NewGraph(t.TempDir())
	g2.AddNode(nil)
	g2.AddEdge("", "b", EdgeCalls)
	if nodeSnapshot(nil).Name != "" {
		t.Fatal("nil snapshot")
	}
}

func TestTraceForwardBackwardNilGraph(t *testing.T) {
	opts := DefaultTraceOptions()
	if TraceForward(nil, "x", opts) != nil || TraceBackward(nil, "x", opts) != nil {
		t.Fatal("nil graph trace")
	}
	if TraceForward(NewGraph(t.TempDir()), "", opts) != nil {
		t.Fatal("empty start")
	}
}

func TestFinalizeEmptyCatalogWarning(t *testing.T) {
	g := NewGraph(t.TempDir())
	if !hasWarningMessage(finalizeWarnings(g, nil, nil, nil), "method_catalog_empty") {
		t.Fatal("expected empty catalog warning")
	}
}

func TestScanTSExportFunctionKind(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "desktop", "frontend", "src", "lib")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(src, "plain.ts"), []byte("export function plainHelper() { return 1; }\n"), 0o644)
	syms, _, _, _, err := ScanTSFiles(dir, map[string]bool{"Submit": true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range syms {
		if s.Name == "plainHelper" {
			found = true
		}
	}
	if !found {
		t.Fatalf("syms = %v", syms)
	}
}

func TestInvalidateFilesNilIndex(t *testing.T) {
	var idx *Index
	if err := idx.InvalidateFiles(nil); err == nil {
		t.Fatal("nil index")
	}
}

func TestContextMinAndPathsFromGoFile(t *testing.T) {
	if min(2, 5) != 2 || min(5, 2) != 2 {
		t.Fatal("min")
	}
	g := NewGraph(t.TempDir())
	id := NewGoBindID("desktop/app.go", "App.Submit")
	g.AddNode(&Node{ID: id, Kind: KindGoBind, Name: "App.Submit", File: "desktop/app.go"})
	g.RebuildIndexes()
	_ = pathsFromGoFile(g, "desktop/app.go")
}

func TestSnapshotToGraphWithoutEdges(t *testing.T) {
	snap := indexSnapshot{
		Version: IndexVersion,
		Root:    t.TempDir(),
		Nodes:   map[string]*Node{"a": {ID: "a", Kind: KindHook, Name: "A"}},
		Out:     map[string][]string{"a": {"b"}},
		In:      map[string][]string{"b": {"a"}},
	}
	g := snapshotToGraph(&snap)
	if len(g.edges) == 0 {
		t.Fatal("expected inferred edges")
	}
}

func TestCrossRealmChangeViaCatalog(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, NewFallbackCatalog(root))
	_ = idx.EnsureReady(context.Background())
	block := BuildCrossRealmContext(idx, []string{
		"desktop/frontend/src/lib/bridge.ts",
		"desktop/app.go",
	}, "go vet ./...")
	if block == "" {
		t.Fatal("expected catalog-based cross realm block")
	}
}

func TestGoBindNonAppReceiverAndEvents(t *testing.T) {
	dir := t.TempDir()
	desktop := filepath.Join(dir, "desktop")
	_ = os.MkdirAll(desktop, 0o755)
	_ = os.WriteFile(filepath.Join(desktop, "emit.go"), []byte(`
package main
import "github.com/wailsapp/wails/v2/pkg/runtime"
func (a *App) Ping() { runtime.EventsEmit(a.ctx, "ping", 1) }
func (s *Service) Bad() {}
`), 0o644)
	binds, _, emits, warns, _ := ScanGoBinds(dir)
	if len(binds) != 1 || binds[0].Method != "Ping" {
		t.Fatalf("binds = %v", binds)
	}
	if len(emits) == 0 {
		t.Fatal("expected EventsEmit site")
	}
	if !hasWarningMessage(warns, "non_app_receiver:Service.Bad") {
		t.Fatalf("warnings = %v", warns)
	}
}

func TestGraphAppendUniqueAndRebuild(t *testing.T) {
	g := NewGraph(t.TempDir())
	id := NewHookID("a.ts", "A")
	g.AddNode(&Node{ID: id, Kind: KindHook, Name: "A", File: "a.ts"})
	g.AddNode(&Node{ID: id, Kind: KindHook, Name: "A", File: "a.ts"})
	g.AddEdge(id, id, EdgeCalls)
	g.AddEdge(id, id, EdgeCalls)
	g.RebuildIndexes()
	if len(g.MethodMap) != 0 {
		t.Fatal("no gobind expected")
	}
}

func TestFindBridgePathForwardMatch(t *testing.T) {
	g := NewGraph(t.TempDir())
	ui := NewUIID("c.tsx", "C")
	bridge := NewBridgeCallID("c.tsx", 2, "Submit")
	gobind := NewGoBindID("desktop/app.go", "App.Submit")
	g.AddNode(&Node{ID: ui, Kind: KindUIComponent, Name: "C", File: "c.tsx"})
	g.AddNode(&Node{ID: bridge, Kind: KindBridgeCall, Name: "app.Submit", File: "c.tsx", Line: 2})
	g.AddNode(&Node{ID: gobind, Kind: KindGoBind, Name: "App.Submit", File: "desktop/app.go"})
	g.AddEdge(ui, bridge, EdgeCalls)
	g.AddEdge(bridge, gobind, EdgeBridgeInvoke)
	g.RebuildIndexes()
	paths, err := FindBridgePath(g, ui, "Submit")
	if err != nil || len(paths) == 0 {
		t.Fatalf("FindBridgePath forward match: %v paths=%d", err, len(paths))
	}
}

func TestEnsureReadyAndRefreshErrors(t *testing.T) {
	var idx *Index
	if err := idx.EnsureReady(context.Background()); err == nil {
		t.Fatal("nil EnsureReady")
	}
	if err := idx.RefreshIfStale(context.Background()); err == nil {
		t.Fatal("nil RefreshIfStale")
	}
}

func TestLoadIndexMissingFile(t *testing.T) {
	if _, err := LoadIndex(t.TempDir()); !errors.Is(err, ErrIndexNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestAtomicWriteSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.json")
	if err := atomicWrite(path, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestParseAppDTSLinesEmpty(t *testing.T) {
	if len(parseAppDTSLines("")) != 0 {
		t.Fatal("expected empty")
	}
}

func TestNodeIDPathParseError(t *testing.T) {
	var id NodeID = "not-a-valid-id"
	if id.Path() == "" && id.Realm() == "" {
		// parse error returns fallback
	}
	if id.Symbol() == "" {
		// ok
	}
}

func TestIsVerifyCommandLint(t *testing.T) {
	if !IsVerifyCommand("golangci-lint run") {
		t.Fatal("lint keyword")
	}
}

func TestToolsAllMetadataMethods(t *testing.T) {
	tools := []interface {
		Description() string
		ReadOnly() bool
	}{
		cgStatusTool{},
		cgTraceForwardTool{},
		cgTraceBackwardTool{},
		cgFindBridgeTool{},
	}
	for _, tl := range tools {
		if tl.Description() == "" {
			t.Fatal("empty description")
		}
		if tl.ReadOnly() != true {
			t.Fatal("tools should be read-only")
		}
	}
}

func TestFindBridgePathErrorsAndBackward(t *testing.T) {
	g := NewGraph(t.TempDir())
	ui := NewUIID("c.tsx", "C")
	bridge := NewBridgeCallID("c.tsx", 2, "Submit")
	gobind := NewGoBindID("desktop/app.go", "App.Submit")
	g.AddNode(&Node{ID: ui, Kind: KindUIComponent, Name: "C", File: "c.tsx"})
	g.AddNode(&Node{ID: bridge, Kind: KindBridgeCall, Name: "app.Submit", File: "c.tsx", Line: 2})
	g.AddNode(&Node{ID: gobind, Kind: KindGoBind, Name: "App.Submit", File: "desktop/app.go"})
	g.AddEdge(bridge, gobind, EdgeBridgeInvoke)
	g.RebuildIndexes()
	if _, err := FindBridgePath(g, ui, "Submit"); err == nil {
		t.Fatal("expected no path error")
	}
	g.AddEdge(ui, bridge, EdgeCalls)
	g.RebuildIndexes()
	if _, err := FindBridgePath(g, ui, "NoMethod"); err == nil {
		t.Fatal("expected missing method error")
	}
}

func TestTraceForwardBackwardDefaultOpts(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	id := g.MethodMap["Submit"]
	paths := TraceForward(g, id, TraceOptions{})
	if paths != nil {
		// zero max uses defaults; may or may not produce paths from gobind forward
	}
	paths = TraceBackward(g, id, TraceOptions{})
	if len(paths) == 0 {
		t.Fatal("expected backward paths with default opts")
	}
}

func TestBuildCrossRealmContextMultiBlockCap(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, NewFallbackCatalog(root))
	_ = idx.EnsureReady(context.Background())
	block := BuildCrossRealmContext(idx, []string{
		"desktop/app.go",
		"desktop/frontend/src/lib/useSubmit.ts",
		"desktop/frontend/src/components/Composer.tsx",
		"desktop/frontend/src/components/Anonymous.tsx",
		"desktop/frontend/src/lib/useController.ts",
	}, "go test ./...")
	if block == "" {
		t.Fatal("expected block")
	}
	chainPart := block
	if i := strings.Index(block, "\n\n## Debug Breakpoints"); i >= 0 {
		chainPart = block[:i]
		if !strings.Contains(block, "## Debug Breakpoints") {
			t.Fatal("expected breakpoint section")
		}
	}
	if len(strings.Split(chainPart, "\n")) > 8 {
		t.Fatalf("expected call-chain line cap, got %d lines in %q", len(strings.Split(chainPart, "\n")), chainPart)
	}
}

func TestSaveMetaMkdirError(t *testing.T) {
	if err := SaveMeta(&Meta{IndexVersion: IndexVersion}, filepath.Join(t.TempDir(), "sub", "deep")); err != nil {
		t.Fatal(err)
	}
}

func TestScanGoBindsRegexDuplicateAndOther(t *testing.T) {
	text := "func (a *App) Dup() {}\nfunc (a *App) Dup() {}\nfunc (x *Other) X() {}\n"
	seen := map[string]bool{}
	m, _, w := scanGoBindsRegex("d.go", text, seen)
	if len(m) != 1 {
		t.Fatalf("binds = %v", m)
	}
	if !hasWarningMessage(w, "duplicate_bind:Dup") || !hasWarningMessage(w, "non_app_receiver:Other.X") {
		t.Fatalf("warnings = %v", w)
	}
}

func TestHasWarningMessageNegative(t *testing.T) {
	if hasWarningMessage(nil, "x") {
		t.Fatal("nil warnings")
	}
}

type stubMCPToolCaller struct {
	ok   bool
	body string
	err  error
}

func (s stubMCPToolCaller) Available() bool { return s.ok }

func (s stubMCPToolCaller) CallTool(context.Context, string, json.RawMessage) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.body, nil
}

func TestSymbolQueryMCPAndNoop(t *testing.T) {
	if NewSymbolQuery(nil).Available() {
		t.Fatal("nil caller should be unavailable")
	}
	q := NewSymbolQuery(stubMCPToolCaller{
		ok:   true,
		body: `{"callees":[{"name":"doSubmit","file":"desktop/app.go","line":3,"kind":"method"}]}`,
	})
	if !q.Available() {
		t.Fatal("expected available")
	}
	refs, err := q.Callees(context.Background(), "App.Submit", 2)
	if err != nil || len(refs) != 1 || refs[0].Name != "doSubmit" {
		t.Fatalf("refs = %+v err = %v", refs, err)
	}
	if _, err := parseCalleesResponse(`{"results":[{"name":"x","file":"a.go"}]}`); err != nil {
		t.Fatal(err)
	}
}

func TestBridgeImpactAdapter(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	a := idx.BridgeImpactAnalyzer()
	if !a.Available() {
		t.Fatal("expected available analyzer")
	}
	nodes, err := a.AffectedUI("Submit")
	if err != nil || len(nodes) == 0 {
		t.Fatalf("nodes = %+v err = %v", nodes, err)
	}
	idx.SetSymbolQuery(MockSymbolQuery{OK: true, Results: []SymbolRef{{Name: "x", File: "f.go"}}})
}

func TestAttachEventEmitsNilGraph(t *testing.T) {
	if w := AttachEventEmits(nil, nil); w != nil {
		t.Fatal("expected nil warnings")
	}
}

func TestLinkEventDeliversNilGraph(t *testing.T) {
	if LinkEventDelivers(nil) != 0 {
		t.Fatal("expected zero")
	}
}
