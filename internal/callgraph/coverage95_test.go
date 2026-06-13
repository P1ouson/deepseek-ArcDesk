package callgraph

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestExtendForwardWithSymbolsBranches(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	from := g.MethodMap["Submit"]
	paths := TraceBackward(g, from, DefaultTraceOptions())
	if len(paths) == 0 {
		t.Fatal("expected backward paths")
	}

	emptySeg := extendForwardWithSymbols(g, []CallPath{{Segments: nil}}, TraceOptions{
		SymbolQuery: MockSymbolQuery{OK: true, Results: []SymbolRef{{Name: "x", File: "f.go"}}},
	})
	if len(emptySeg) != 0 {
		t.Fatalf("emptySeg = %v", emptySeg)
	}

	errOpts := TraceOptions{
		MaxPaths:    3,
		SymbolQuery: MockSymbolQuery{OK: true, Err: errors.New("fail")},
	}
	gobindPath := []CallPath{{Segments: []PathSegment{{Node: NodeSnapshot{
		ID: from, Kind: KindGoBind, Name: "App.Submit", File: "desktop/app.go",
	}}}}}
	if got := extendForwardWithSymbols(g, gobindPath, errOpts); len(got) != 1 {
		t.Fatalf("expected original path on symbol error, got %v", got)
	}

	opts := TraceOptions{
		MaxPaths: 1,
		SymbolQuery: MockSymbolQuery{OK: true, Results: []SymbolRef{
			{Name: "helper1", File: "desktop/app.go", Line: 1},
			{Name: "helper2", File: "desktop/app.go", Line: 2},
		}},
	}
	ext := extendForwardWithSymbols(g, paths, opts)
	if len(ext) == 0 {
		t.Fatal("expected extended paths")
	}

	g2 := NewGraph(t.TempDir())
	hook := NewHookID("a.ts", "useX")
	g2.AddNode(&Node{ID: hook, Kind: KindHook, Name: "useX", File: "a.ts"})
	hookPaths := []CallPath{{Segments: []PathSegment{{Node: nodeSnapshot(g2.Nodes[hook])}}}}
	if got := extendForwardWithSymbols(g2, hookPaths, TraceOptions{
		MaxPaths:    3,
		SymbolQuery: MockSymbolQuery{OK: true, Results: []SymbolRef{{Name: "x"}}},
	}); len(got) != 1 {
		t.Fatalf("non-gobind ext = %v", got)
	}
}

func TestAttachEventEmitsCreatesPackageNode(t *testing.T) {
	g := NewGraph(t.TempDir())
	w := AttachEventEmits(g, []EventEmitSite{
		{Channel: "test:event", File: "desktop/app.go", Line: 1},
	})
	if len(w) != 0 {
		t.Fatalf("warnings = %v", w)
	}
	pkgID := NewGoInternalID("desktop/app.go", "package")
	if _, ok := g.Nodes[pkgID]; !ok {
		t.Fatal("expected package node")
	}
}

func TestAttachEventEmitsSkipsEmpty(t *testing.T) {
	g := NewGraph(t.TempDir())
	if w := AttachEventEmits(g, []EventEmitSite{{File: ""}, {File: "f.go", Channel: ""}}); w != nil {
		t.Fatal("expected nil warnings")
	}
}

func TestLinkListenEdgesWithScope(t *testing.T) {
	g := NewGraph(t.TempDir())
	listen := NewEventListenID("a.ts", 1, "ch")
	scope := NewUIID("a.ts", "App")
	g.AddNode(&Node{ID: listen, Kind: KindEventListen, Name: "ch", File: "a.ts"})
	g.AddNode(&Node{ID: scope, Kind: KindUIComponent, Name: "App", File: "a.ts"})
	LinkListenEdges(g, []TSListen{{ID: listen, Scope: scope}})
	if g.EdgeCount() != 1 {
		t.Fatal("expected edge")
	}
	LinkListenEdges(g, []TSListen{{ID: listen, Scope: ""}})
	LinkListenEdges(nil, nil)
}

func TestNewSymbolQueryFromRegistry(t *testing.T) {
	if NewSymbolQueryFromRegistry(nil).Available() {
		t.Fatal("nil registry should be unavailable")
	}
	reg := tool.NewRegistry()
	reg.Add(mockCodegraphTool{})
	q := NewSymbolQueryFromRegistry(reg)
	if !q.Available() {
		t.Fatal("expected available")
	}
	refs, err := q.Callees(context.Background(), "App.Submit", 0)
	if err != nil || len(refs) != 1 || refs[0].Name != "helper" {
		t.Fatalf("refs=%v err=%v", refs, err)
	}
}

type mockCodegraphTool struct{}

func (mockCodegraphTool) Name() string        { return codegraphCalleesTool }
func (mockCodegraphTool) Description() string { return "mock" }
func (mockCodegraphTool) Schema() json.RawMessage {
	return json.RawMessage(`{}`)
}
func (mockCodegraphTool) ReadOnly() bool { return true }
func (mockCodegraphTool) Execute(context.Context, json.RawMessage) (string, error) {
	return `{"callees":[{"name":"helper","file":"a.go","line":1}]}`, nil
}

func TestMcpSymbolQueryCallToolError(t *testing.T) {
	q := NewSymbolQuery(stubMCPToolCaller{ok: true, err: errors.New("mcp fail")})
	if _, err := q.Callees(context.Background(), "x", 2); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseCalleesResponseEmpty(t *testing.T) {
	refs, err := parseCalleesResponse("")
	if err != nil || refs != nil {
		t.Fatalf("refs=%v err=%v", refs, err)
	}
}

func TestLoadIndexCorruptCallgraph(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, indexFileName), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIndex(dir); !errors.Is(err, ErrIndexCorrupt) {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadMetaCorruptCallgraph(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, metaFileName), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMeta(dir); !errors.Is(err, ErrIndexCorrupt) {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenCorruptIndex(t *testing.T) {
	root := copyWailsTestProject(t)
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, indexFileName), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root, nil); err == nil {
		t.Fatal("expected corrupt index error")
	}
}

func TestAtomicWriteFileFailure(t *testing.T) {
	old := atomicWriteFile
	defer func() { atomicWriteFile = old }()
	atomicWriteFile = func(string, []byte) error { return errors.New("disk full") }
	if err := atomicWrite(filepath.Join(t.TempDir(), "x.json"), []byte("{}")); err == nil {
		t.Fatal("expected write error")
	}
}

func TestAtomicWriteRenameFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dest.json")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(path, []byte("{}")); err == nil {
		t.Fatal("expected rename error")
	}
}

func TestFallbackCatalogBridgeModuleKind(t *testing.T) {
	cat := NewFallbackCatalog(t.TempDir())
	if kind, ok := cat.ModuleKind("bridge:Submit"); !ok || kind != "bridge" {
		t.Fatalf("kind=%q ok=%v", kind, ok)
	}
	if _, ok := cat.ModuleKind(""); ok {
		t.Fatal("empty module")
	}
}

func TestParseNodeIDInvalidRealm(t *testing.T) {
	if _, err := ParseNodeID("npm:foo"); err == nil {
		t.Fatal("expected invalid realm")
	}
}

func TestGraphRebuildDuplicateBridge(t *testing.T) {
	g := NewGraph(t.TempDir())
	b1 := NewBridgeCallID("a.tsx", 1, "Submit")
	b2 := NewBridgeCallID("b.tsx", 2, "Submit")
	g.AddNode(&Node{ID: b1, Kind: KindBridgeCall, Name: "app.Submit", File: "a.tsx"})
	g.AddNode(&Node{ID: b2, Kind: KindBridgeCall, Name: "app.Submit", File: "b.tsx"})
	g.AddNode(&Node{ID: NewGoBindID("app.go", "App.Submit"), Kind: KindGoBind, Name: "App.Submit", File: "app.go"})
	g.RebuildIndexes()
	if len(g.BridgeByMethod["Submit"]) != 2 {
		t.Fatalf("bridges=%v", g.BridgeByMethod["Submit"])
	}
}

func TestAppendUniqueDuplicate(t *testing.T) {
	id := NodeID("x")
	list := appendUnique([]NodeID{id}, id)
	if len(list) != 1 {
		t.Fatal("duplicate should not append")
	}
}

func TestIndexFindBridgeByLine(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	paths, err := idx.FindBridge(context.Background(), "desktop/frontend/src/lib/useSubmit.ts", 5, "Submit")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("expected bridge paths")
	}
}

func TestIndexTraceNotReady(t *testing.T) {
	idx := &Index{root: t.TempDir()}
	if _, err := idx.TraceForward(context.Background(), "x", "y", TraceOptions{}); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("forward err=%v", err)
	}
	if _, err := idx.TraceBackward(context.Background(), "Missing", TraceOptions{}); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("backward err=%v", err)
	}
	if _, err := idx.FindBridge(context.Background(), "x", 0, "Submit"); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("bridge err=%v", err)
	}
}

func TestSetSymbolQueryNilIndex(t *testing.T) {
	var idx *Index
	idx.SetSymbolQuery(MockSymbolQuery{OK: true})
}

func TestBuildCrossRealmContextMultipleBlocks(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, NewFallbackCatalog(root))
	_ = idx.EnsureReady(context.Background())
	block := BuildCrossRealmContext(idx, []string{
		"desktop/app.go",
		"desktop/frontend/src/lib/useSubmit.ts",
		"desktop/frontend/src/components/Composer.tsx",
		"desktop/frontend/src/components/Anonymous.tsx",
	}, "npm test")
	if block == "" || !strings.Contains(block, "## Wails Call Chain") {
		t.Fatalf("block = %q", block)
	}
}

func TestCrossRealmChangeCatalogPaths(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, NewFallbackCatalog(root))
	if !crossRealmChange(idx, []string{"desktop/frontend/src/lib/bridge.ts", "desktop/app.go"}) {
		t.Fatal("expected cross realm via catalog")
	}
}

func TestBreakpointsFromPathSkipsEmptyNode(t *testing.T) {
	if bps := BreakpointsFromPath(CallPath{Segments: []PathSegment{{Node: NodeSnapshot{}}}}); len(bps) != 0 {
		t.Fatalf("bps=%v", bps)
	}
}

func TestAutoBreakpointsNonCrossRealm(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	bps, err := idx.AutoBreakpoints(context.Background(), []string{"desktop/app.go"}, "")
	if err != nil || bps != nil {
		t.Fatalf("bps=%v err=%v", bps, err)
	}
}

func TestCgTraceForwardToolNotReady(t *testing.T) {
	toolDef := cgTraceForwardTool{idx: &Index{root: t.TempDir()}}
	out, err := toolDef.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "not ready") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestCgFindBridgeToolExecute(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	findTool, _ := reg.Get("callgraph_find_bridge")
	out, err := findTool.Execute(context.Background(), json.RawMessage(`{"frontend":"desktop/frontend/src/lib/useSubmit.ts#useSubmit","go_method":"Submit"}`))
	if err != nil || !strings.Contains(out, "Submit") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestSaveMetaEmptyDir(t *testing.T) {
	if err := SaveMeta(&Meta{IndexVersion: IndexVersion}, ""); err == nil {
		t.Fatal("expected empty dir error")
	}
}

func TestGraphAddNodeRecreatesMap(t *testing.T) {
	g := &CallGraph{}
	g.AddNode(&Node{ID: "hook:a", Kind: KindHook, Name: "A"})
	if g.Nodes == nil || g.Nodes["hook:a"] == nil {
		t.Fatal("expected node after nil map init")
	}
}

func TestGraphAddEdgeDuplicate(t *testing.T) {
	g := NewGraph(t.TempDir())
	a, b := NewHookID("a.ts", "A"), NewHookID("b.ts", "B")
	g.AddNode(&Node{ID: a, Kind: KindHook, Name: "A", File: "a.ts"})
	g.AddNode(&Node{ID: b, Kind: KindHook, Name: "B", File: "b.ts"})
	g.AddEdge(a, b, EdgeCalls)
	g.AddEdge(a, b, EdgeCalls)
	if g.EdgeCount() != 1 {
		t.Fatalf("edges = %d", g.EdgeCount())
	}
}

func TestRegistryToolCallerUnavailable(t *testing.T) {
	var r registryToolCaller
	if r.Available() {
		t.Fatal("nil registry unavailable")
	}
	if _, err := r.CallTool(context.Background(), "", nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestNoopSymbolQueryCallees(t *testing.T) {
	var q noopSymbolQuery
	if q.Available() {
		t.Fatal("noop unavailable")
	}
	if _, err := q.Callees(context.Background(), "x", 1); err == nil {
		t.Fatal("expected error")
	}
}

func TestMcpSymbolQueryUnavailable(t *testing.T) {
	var q *mcpSymbolQuery
	if _, err := q.Callees(context.Background(), "x", 1); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseNodeIDValidHook(t *testing.T) {
	id, err := ParseNodeID("hook:useSubmit")
	if err != nil || id.Realm() != "hook" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}

func TestFallbackCatalogResolveMissingFile(t *testing.T) {
	cat := NewFallbackCatalog(t.TempDir())
	if _, ok := cat.ResolveFile("missing.go"); ok {
		t.Fatal("expected missing file")
	}
}

func TestBuildCrossRealmContextForwardResolve(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, NewFallbackCatalog(root))
	_ = idx.EnsureReady(context.Background())
	block := BuildCrossRealmContext(idx, []string{
		"desktop/frontend/src/components/Composer.tsx",
		"desktop/app.go",
	}, "go test ./...")
	if block == "" || !strings.Contains(block, "## Wails Call Chain") {
		t.Fatalf("block = %q", block)
	}
}

func TestBuildCrossRealmContextIndexNotReady(t *testing.T) {
	root := copyWailsTestProject(t)
	idx := &Index{root: root, catalog: NewFallbackCatalog(root)}
	if got := BuildCrossRealmContext(idx, []string{
		"desktop/app.go",
		"desktop/frontend/src/lib/useSubmit.ts",
	}, "go test"); got != "" {
		t.Fatalf("got = %q", got)
	}
}

func TestBreakpointsForQueryPathAndMethod(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	bps, paths, err := idx.BreakpointsForQuery(context.Background(),
		"desktop/frontend/src/lib/useSubmit.ts", "useSubmit", "Submit")
	if err != nil || len(bps) == 0 || len(paths) == 0 {
		t.Fatalf("bps=%d paths=%d err=%v", len(bps), len(paths), err)
	}
}

func TestExtendForwardEmptyCallees(t *testing.T) {
	g := NewGraph(t.TempDir())
	from := NewGoBindID("app.go", "App.Submit")
	paths := []CallPath{{Segments: []PathSegment{{Node: NodeSnapshot{
		ID: from, Kind: KindGoBind, Name: "App.Submit", File: "app.go",
	}}}}}
	got := extendForwardWithSymbols(g, paths, TraceOptions{
		MaxPaths:    3,
		SymbolQuery: MockSymbolQuery{OK: true, Results: nil},
	})
	if len(got) != 1 {
		t.Fatalf("got=%v", got)
	}
}

func TestCgTraceBackwardToolExecute(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	toolDef := cgTraceBackwardTool{idx: idx}
	out, err := toolDef.Execute(context.Background(), json.RawMessage(`{"go_method":"Submit"}`))
	if err != nil || !strings.Contains(out, "Submit") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestCgStatusToolReady(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	toolDef := cgStatusTool{idx: idx}
	out, err := toolDef.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "Callgraph index") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestCrossRealmImpactAppPrefix(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	if _, err := idx.CrossRealmImpact("App.Submit"); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCorruptMeta(t *testing.T) {
	root := copyWailsTestProject(t)
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveIndex(g, dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, metaFileName), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root, nil); err == nil {
		t.Fatal("expected corrupt meta error")
	}
}

func TestAutoBreakpointsCrossRealmWithMethod(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	bps, err := idx.AutoBreakpoints(context.Background(), []string{
		"desktop/app.go",
		"desktop/frontend/src/lib/useSubmit.ts",
	}, "Submit")
	if err != nil || len(bps) == 0 {
		t.Fatalf("bps=%v err=%v", bps, err)
	}
}

func TestParseCalleesResponseInvalidJSON(t *testing.T) {
	if _, err := parseCalleesResponse("not-json"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestMcpSymbolQueryZeroDepth(t *testing.T) {
	q := NewSymbolQuery(stubMCPToolCaller{
		ok:   true,
		body: `{"callees":[{"name":"x","file":"a.go","line":1}]}`,
	})
	refs, err := q.Callees(context.Background(), "App.X", 0)
	if err != nil || len(refs) != 1 {
		t.Fatalf("refs=%v err=%v", refs, err)
	}
}

func TestLoadIndexCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, indexFileName), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIndex(dir); !errors.Is(err, ErrIndexCorrupt) {
		t.Fatalf("err = %v", err)
	}
}

func TestComputeFingerprintWailsProject(t *testing.T) {
	root := copyWailsTestProject(t)
	if fp := ComputeFingerprint(root); fp == "" {
		t.Fatal("expected fingerprint")
	}
}

func TestNewMetaGitHead(t *testing.T) {
	meta := NewMeta(copyWailsTestProject(t))
	if meta.Fingerprint == "" || meta.IndexVersion != IndexVersion {
		t.Fatalf("meta=%+v", meta)
	}
}

func TestFindBridgePathBackwardOnly(t *testing.T) {
	g := NewGraph(t.TempDir())
	frontend := NewHookID("useSubmit.ts", "useSubmit")
	bridge := NewBridgeCallID("useSubmit.ts", 2, "Submit")
	gobind := NewGoBindID("app.go", "App.Submit")
	g.AddNode(&Node{ID: frontend, Kind: KindHook, Name: "useSubmit", File: "useSubmit.ts"})
	g.AddNode(&Node{ID: bridge, Kind: KindBridgeCall, Name: "app.Submit", File: "useSubmit.ts", Line: 2})
	g.AddNode(&Node{ID: gobind, Kind: KindGoBind, Name: "App.Submit", File: "app.go"})
	g.AddEdge(frontend, bridge, EdgeCalls)
	g.AddEdge(bridge, gobind, EdgeBridgeInvoke)
	g.RebuildIndexes()
	paths, err := FindBridgePath(g, frontend, "Submit")
	if err != nil || len(paths) == 0 {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
}

func TestBuildCrossRealmContextSkipsBlankPaths(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, NewFallbackCatalog(root))
	_ = idx.EnsureReady(context.Background())
	block := BuildCrossRealmContext(idx, []string{"  ", "desktop/app.go", "desktop/frontend/src/lib/useSubmit.ts"}, "go test")
	if block == "" {
		t.Fatal("expected block")
	}
}

func TestCrossRealmChangeGoModPath(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, NewFallbackCatalog(root))
	if !crossRealmChange(idx, []string{"desktop/app.go", "desktop/frontend/src/lib/bridge.ts"}) {
		t.Fatal("expected go + js cross realm")
	}
}

func TestNodeIDForKindFunction(t *testing.T) {
	if got := nodeIDForKind(KindTSFunction, "f.ts", "helper"); got != NewFnID("f.ts", "helper") {
		t.Fatalf("got=%q", got)
	}
}

func TestOpenIndexWithoutMeta(t *testing.T) {
	root := copyWailsTestProject(t)
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveIndex(g, dir); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(dir, metaFileName))
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestReceiverTypeNameValueReceiver(t *testing.T) {
	dir := t.TempDir()
	desktop := filepath.Join(dir, "desktop")
	_ = os.MkdirAll(desktop, 0o755)
	_ = os.WriteFile(filepath.Join(desktop, "val.go"), []byte("package main\nfunc (a App) Val() {}\n"), 0o644)
	binds, _, _, _, _ := ScanGoBinds(dir)
	if len(binds) == 0 {
		t.Fatal("expected value receiver bind")
	}
}

func TestFormatCrossRealmContextMultiplePaths(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	from, err := ResolveNodeID(g, "desktop/frontend/src/lib/useSubmit.ts", "useSubmit")
	if err != nil {
		t.Fatal(err)
	}
	paths := TraceForward(g, from, DefaultTraceOptions())
	if len(paths) < 2 {
		paths = append(paths, paths[0])
	}
	out := FormatCrossRealmContext(paths, "Submit")
	if out == "" || !strings.Contains(out, "Submit") {
		t.Fatalf("out=%q", out)
	}
}

func TestIndexTraceForwardWithInjectedSymbolQuery(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	idx.SetSymbolQuery(MockSymbolQuery{OK: true, Results: []SymbolRef{{Name: "doSubmit", File: "desktop/app.go", Line: 1}}})
	paths, err := idx.TraceForward(context.Background(), "desktop/frontend/src/lib/useSubmit.ts", "useSubmit", TraceOptions{IncludeGoInternal: true, MaxPaths: 3})
	if err != nil || len(paths) == 0 {
		t.Fatalf("paths=%d err=%v", len(paths), err)
	}
}

func TestCgFindBridgeToolMissingMethod(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	toolDef := cgFindBridgeTool{idx: idx}
	out, err := toolDef.Execute(context.Background(), json.RawMessage(`{"frontend":"desktop/frontend/src/lib/useSubmit.ts","go_method":"Missing"}`))
	if err == nil {
		t.Fatalf("expected error, out=%q", out)
	}
}

func TestBuildMethodCatalogMissingWailsJS(t *testing.T) {
	root := t.TempDir()
	desktop := filepath.Join(root, "desktop")
	if err := os.MkdirAll(desktop, 0o755); err != nil {
		t.Fatal(err)
	}
	res := BuildMethodCatalog(root, []GoBindMethod{{Method: "Ping", ID: NewGoBindID("desktop/app.go", "App.Ping")}})
	if !hasWarningMessage(res.Warnings, "wailsjs_missing") {
		t.Fatalf("warnings=%v", res.Warnings)
	}
	if !res.Methods["Ping"] {
		t.Fatal("expected bind method in catalog")
	}
}

func TestBuildMethodCatalogBrokenDTSSalvage(t *testing.T) {
	root := copyWailsTestProject(t)
	_ = os.Remove(filepath.Join(root, "desktop", "frontend", "wailsjs", "go", "main", "App.d.ts"))
	res := BuildMethodCatalog(root, nil)
	if len(res.Methods) == 0 {
		t.Fatal("expected salvaged methods")
	}
}

func TestLinkBridgeEdgesOrphanWarning(t *testing.T) {
	g := NewGraph(t.TempDir())
	orphan := NewBridgeCallID("a.tsx", 1, "Orphan")
	g.AddNode(&Node{ID: orphan, Kind: KindBridgeCall, Name: "app.Orphan", File: "a.tsx", Line: 1})
	w := LinkBridgeEdges(g, nil)
	if !hasWarningMessage(w, "orphan_bridge:Orphan") {
		t.Fatalf("warnings=%v", w)
	}
}

func TestParseEventsEmitEmptyChannelLiteral(t *testing.T) {
	dir := t.TempDir()
	desktop := filepath.Join(dir, "desktop")
	_ = os.MkdirAll(desktop, 0o755)
	src := `package main
import "github.com/wailsapp/wails/v2/pkg/runtime"
func (a *App) Ping() { runtime.EventsEmit(a.ctx, "", 1) }
`
	_ = os.WriteFile(filepath.Join(desktop, "emit.go"), []byte(src), 0o644)
	_, _, emits, _, _ := ScanGoBinds(dir)
	if len(emits) != 0 {
		t.Fatalf("emits=%v", emits)
	}
}

func TestNodeIDRealmInvalid(t *testing.T) {
	var bad NodeID = "not-valid"
	if bad.Realm() != "" {
		t.Fatal("invalid realm should be empty")
	}
}

func TestCheckStaleMatchingGitHead(t *testing.T) {
	root := copyWailsTestProject(t)
	meta := NewMeta(root)
	meta.GitHead = gitHead(root)
	if meta.GitHead != "" && CheckStale(root, meta) {
		t.Fatal("matching git head should not be stale by head alone")
	}
}

func TestSaveLoadIndexRoundTripEdges(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveIndex(g, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadIndex(dir)
	if err != nil || loaded.EdgeCount() == 0 {
		t.Fatalf("loaded edges=%d err=%v", loaded.EdgeCount(), err)
	}
}

func TestCgTraceForwardToolWithSymbolInFrom(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	toolDef := cgTraceForwardTool{idx: idx}
	out, err := toolDef.Execute(context.Background(), json.RawMessage(`{"from":"desktop/frontend/src/lib/useSubmit.ts#useSubmit","include_go_internal":true}`))
	if err != nil || !strings.Contains(out, "Trace forward") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestAutoBreakpointsForwardResolvedPath(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	bps, err := idx.AutoBreakpoints(context.Background(), []string{
		"desktop/frontend/src/lib/useSubmit.ts",
		"desktop/app.go",
	}, "")
	if err != nil || len(bps) == 0 {
		t.Fatalf("bps=%v err=%v", bps, err)
	}
}

func TestBreakpointsFromPathDuplicateLayer(t *testing.T) {
	path := CallPath{Segments: []PathSegment{
		{Node: NodeSnapshot{ID: "1", Kind: KindHook, Name: "a", File: "a.ts", Line: 1}, Edge: EdgeCalls},
		{Node: NodeSnapshot{ID: "2", Kind: KindUIComponent, Name: "b", File: "b.tsx", Line: 2}, Edge: EdgeCalls},
		{Node: NodeSnapshot{ID: "3", Kind: KindUIHandler, Name: "c", File: "c.tsx", Line: 3}, Edge: EdgeCalls},
	}}
	bps := BreakpointsFromPath(path)
	if len(bps) != 1 {
		t.Fatalf("bps=%v", bps)
	}
}

func TestCrossRealmImpactNotReady(t *testing.T) {
	idx := &Index{root: t.TempDir()}
	if _, err := idx.CrossRealmImpact("Submit"); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("err=%v", err)
	}
}

func TestBuildGraphEmptyRootError(t *testing.T) {
	if _, _, err := BuildGraph(BuildOptions{Root: ""}); err == nil {
		t.Fatal("expected empty root error")
	}
}

func TestFindBridgePathNilGraph(t *testing.T) {
	if _, err := FindBridgePath(nil, "x", "Submit"); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("err=%v", err)
	}
}

func TestFindBridgePathForwardDirect(t *testing.T) {
	g := NewGraph(t.TempDir())
	frontend := NewHookID("useSubmit.ts", "useSubmit")
	bridge := NewBridgeCallID("useSubmit.ts", 2, "Submit")
	gobind := NewGoBindID("app.go", "App.Submit")
	g.AddNode(&Node{ID: frontend, Kind: KindHook, Name: "useSubmit", File: "useSubmit.ts"})
	g.AddNode(&Node{ID: bridge, Kind: KindBridgeCall, Name: "app.Submit", File: "useSubmit.ts", Line: 2})
	g.AddNode(&Node{ID: gobind, Kind: KindGoBind, Name: "App.Submit", File: "app.go"})
	g.AddEdge(frontend, bridge, EdgeCalls)
	g.AddEdge(bridge, gobind, EdgeBridgeInvoke)
	g.RebuildIndexes()
	paths, err := FindBridgePath(g, frontend, "Submit")
	if err != nil || len(paths) == 0 {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
}

func TestBuildCrossRealmContextResolvedSingleNodeFile(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, NewFallbackCatalog(root))
	_ = idx.EnsureReady(context.Background())
	block := BuildCrossRealmContext(idx, []string{
		"desktop/frontend/src/lib/useSubmit.ts",
		"desktop/app.go",
	}, "pnpm test")
	if block == "" {
		t.Fatal("expected cross-realm block")
	}
}

func TestCgBreakpointsToolChangedPaths(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	toolDef := cgBreakpointsTool{idx: idx}
	out, err := toolDef.Execute(context.Background(), json.RawMessage(`{"changed_paths":["desktop/app.go","desktop/frontend/src/lib/useSubmit.ts"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Debug") && !strings.Contains(out, "No cross-realm") {
		t.Fatalf("out=%q", out)
	}
}

func TestIndexRefreshAfterInvalidate(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	_ = idx.InvalidateFiles([]string{"desktop/app.go"})
	if err := idx.RefreshIfStale(context.Background()); err != nil {
		t.Fatal(err)
	}
	stats, err := idx.Status()
	if err != nil || stats.Stale {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
}

func TestTraceForwardStopAtGoBind(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	from, err := ResolveNodeID(g, "desktop/frontend/src/lib/useSubmit.ts", "useSubmit")
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultTraceOptions()
	opts.StopAtGoBindForward = true
	opts.MaxPaths = 5
	if paths := TraceForward(g, from, opts); len(paths) == 0 {
		t.Fatal("expected forward paths")
	}
}

func TestAllNodeIDRealmsAndParse(t *testing.T) {
	cases := []struct {
		id     NodeID
		realm  string
		symbol string
	}{
		{NewUIID("a.tsx", "App"), "ui", "App"},
		{NewHookID("a.ts", "useX"), "hook", "useX"},
		{NewFnID("a.ts", "fn"), "fn", "fn"},
		{NewBridgeCallID("a.tsx", 1, "Submit"), "bridge", "app.Submit"},
		{NewGoBindID("a.go", "App.Submit"), "gobind", "App.Submit"},
		{NewGoInternalID("a.go", "helper"), "go", "helper"},
		{NewEventEmitID("a.go", 1, "ch"), "emit", "ch"},
		{NewEventListenID("a.ts", 2, "ch"), "listen", "ch"},
	}
	for _, tc := range cases {
		parsed, err := ParseNodeID(string(tc.id))
		if err != nil {
			t.Fatalf("ParseNodeID(%q) = %v", tc.id, err)
		}
		if parsed.Realm() != tc.realm || parsed.Symbol() != tc.symbol || parsed.Path() == "" {
			t.Fatalf("id=%q realm=%q symbol=%q path=%q", tc.id, parsed.Realm(), parsed.Symbol(), parsed.Path())
		}
	}
}

func TestFinalizeWarningsUnusedBindAndDrift(t *testing.T) {
	g := NewGraph(t.TempDir())
	g.BridgeByMethod = map[string][]NodeID{"Used": {NewBridgeCallID("a.tsx", 1, "Used")}}
	binds := []GoBindMethod{
		{Method: "Used", File: "a.go", ID: NewGoBindID("a.go", "App.Used")},
		{Method: "Unused", File: "a.go", ID: NewGoBindID("a.go", "App.Unused")},
	}
	dts := map[string]bool{"Used": true, "Extra": true}
	w := finalizeWarnings(g, binds, map[string]bool{"Used": true}, dts)
	if !hasWarningMessage(w, "unused_bind:Unused") || !hasWarningMessage(w, "app_dts_drift:extra:Extra") {
		t.Fatalf("warnings=%v", w)
	}
}

func TestParseAppBindingsFromFixture(t *testing.T) {
	root := copyWailsTestProject(t)
	methods, err := ParseAppBindings(root)
	if err != nil || len(methods) == 0 {
		t.Fatalf("methods=%v err=%v", methods, err)
	}
}

func TestScanBindBodyCallsNilBody(t *testing.T) {
	if got := scanBindBodyCalls(nil, "a.go", NewGoBindID("a.go", "App.X"), nil); len(got) != 0 {
		t.Fatalf("got=%v", got)
	}
}

func TestProjectDirCreatesDir(t *testing.T) {
	root := t.TempDir()
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("dir=%q err=%v", dir, err)
	}
}

func TestRegistryToolCallerExecute(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(mockCodegraphTool{})
	r := registryToolCaller{reg: reg}
	out, err := r.CallTool(context.Background(), "", json.RawMessage(`{"symbol":"App.X"}`))
	if err != nil || !strings.Contains(out, "helper") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestWailsProjectFullSurface(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, NewFallbackCatalog(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	idx.SetSymbolQuery(MockSymbolQuery{
		OK:      true,
		Results: []SymbolRef{{Name: "doSubmit", File: "desktop/app.go", Line: 1, Kind: "method"}},
	})

	ctx := context.Background()
	for _, method := range []string{"Submit", "Notify", "App.Submit"} {
		if _, err := idx.TraceBackward(ctx, method, TraceOptions{IncludeEvents: true, IncludeGoInternal: true, MaxPaths: 3}); err != nil && !errors.Is(err, ErrNodeNotFound) {
			t.Fatalf("TraceBackward(%q): %v", method, err)
		}
	}
	if _, err := idx.TraceForward(ctx, "desktop/frontend/src/lib/useSubmit.ts", "useSubmit", TraceOptions{IncludeGoInternal: true, MaxPaths: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.CrossRealmImpact("Submit"); err != nil {
		t.Fatal(err)
	}
	if _, err := idx.AutoBreakpoints(ctx, []string{"desktop/app.go", "desktop/frontend/src/lib/useSubmit.ts"}, "Submit"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := idx.BreakpointsForQuery(ctx, "desktop/frontend/src/lib/useSubmit.ts", "useSubmit", "Submit"); err != nil {
		t.Fatal(err)
	}
	block := BuildCrossRealmContext(idx, []string{
		"desktop/app.go",
		"desktop/frontend/src/lib/useSubmit.ts",
		"desktop/frontend/src/components/Composer.tsx",
		"desktop/frontend/src/lib/bridge.ts",
	}, "go test ./...")
	if block == "" {
		t.Fatal("expected cross-realm context")
	}
	_ = idx.InvalidateFiles([]string{"desktop/app.go"})
	if err := idx.RefreshIfStale(ctx); err != nil {
		t.Fatal(err)
	}

	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	for _, name := range []string{"callgraph_status", "callgraph_trace_forward", "callgraph_trace_backward", "callgraph_find_bridge", "callgraph_breakpoints"} {
		toolDef, ok := reg.Get(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		_, _ = toolDef.Execute(ctx, json.RawMessage(`{"from":"desktop/frontend/src/lib/useSubmit.ts#useSubmit","go_method":"Submit","changed_paths":["desktop/app.go","desktop/frontend/src/lib/useSubmit.ts"]}`))
	}
}

func TestComputeStatsSkipsNilNode(t *testing.T) {
	g := NewGraph(t.TempDir())
	g.AddNode(&Node{ID: NewBridgeCallID("a.tsx", 1, "X"), Kind: KindBridgeCall, Name: "app.X", File: "a.tsx"})
	g.Nodes["bad"] = nil
	stats := computeStats(g, 0, 0)
	if stats.BridgeCallCount != 1 {
		t.Fatalf("stats=%+v", stats)
	}
}

func TestLinkEventDeliversSkipsNilNode(t *testing.T) {
	g := NewGraph(t.TempDir())
	emit := NewEventEmitID("a.go", 1, "ch")
	g.AddNode(&Node{ID: emit, Kind: KindEventEmit, Name: "ch", File: "a.go"})
	g.Nodes["nil"] = nil
	if LinkEventDelivers(g) != 0 {
		t.Fatal("expected zero delivers")
	}
}

func TestFormatCrossRealmContextRPCAndEvent(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	rpc := TraceBackward(g, g.MethodMap["Submit"], DefaultTraceOptions())
	eventOpts := DefaultTraceOptions()
	eventOpts.IncludeEvents = true
	event := TraceBackward(g, g.MethodMap["Notify"], eventOpts)
	out := FormatCrossRealmContext(append(rpc, event...), "Submit")
	if !strings.Contains(out, "Event") && !strings.Contains(out, "Submit") {
		t.Fatalf("out=%q", out)
	}
}

func TestScanTSSourceFileReadError(t *testing.T) {
	_, _, _, warns := scanTSSourceFile("missing.ts", filepath.Join(t.TempDir(), "missing.ts"), nil)
	if len(warns) == 0 || !strings.Contains(warns[0].Message, "ts_read_error") {
		t.Fatalf("warns=%v", warns)
	}
}

func TestScanTSSkipsTestFileSuffix(t *testing.T) {
	root := t.TempDir()
	srcRoot := filepath.Join(root, "desktop", "frontend", "src")
	if err := os.MkdirAll(srcRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(srcRoot, "sample.test.ts")
	if err := os.WriteFile(path, []byte("export function SampleTest() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	symbols, _, _, _, err := ScanTSFiles(root, map[string]bool{"Submit": true})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range symbols {
		if strings.Contains(s.File, ".test.ts") {
			t.Fatalf("unexpected test file symbol: %+v", s)
		}
	}
}

func TestScanTSEventsOnConstChannel(t *testing.T) {
	dir := t.TempDir()
	relDir := filepath.Join(dir, "desktop", "frontend", "src", "lib")
	if err := os.MkdirAll(relDir, 0o755); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(relDir, "events_bridge.ts")
	src := `const CHANNEL = "agent:event";
export function onAgentEvent() {
  EventsOn(CHANNEL, () => {});
}
`
	if err := os.WriteFile(abs, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, listens, _, err := ScanTSFiles(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range listens {
		if l.Channel == "agent:event" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("listens=%v", listens)
	}
}

func TestScanTSMockRegion(t *testing.T) {
	dir := t.TempDir()
	relDir := filepath.Join(dir, "desktop", "frontend", "src", "lib")
	if err := os.MkdirAll(relDir, 0o755); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(relDir, "mock.ts")
	src := `// --- browser dev mock
ignored code
export function Visible() {}
`
	if err := os.WriteFile(abs, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	symbols, _, _, _, err := ScanTSFiles(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) == 0 {
		t.Fatal("expected visible symbol outside mock region")
	}
}

func TestBuildCrossRealmContextTraceForwardLoop(t *testing.T) {
	root := t.TempDir()
	g := NewGraph(root)
	uiFile := "desktop/frontend/src/components/Solo.tsx"
	ui := NewUIID(uiFile, "Solo")
	bridge := NewBridgeCallID(uiFile, 5, "Submit")
	gobind := NewGoBindID("desktop/app.go", "App.Submit")
	g.AddNode(&Node{ID: ui, Kind: KindUIComponent, Name: "Solo", File: uiFile, Line: 1})
	g.AddNode(&Node{ID: bridge, Kind: KindBridgeCall, Name: "app.Submit", File: uiFile, Line: 5})
	g.AddNode(&Node{ID: gobind, Kind: KindGoBind, Name: "App.Submit", File: "desktop/app.go"})
	g.AddEdge(ui, bridge, EdgeCalls)
	g.AddEdge(bridge, gobind, EdgeBridgeInvoke)
	g.RebuildIndexes()
	idx := &Index{
		root:    root,
		graph:   g,
		meta:    &Meta{IndexVersion: IndexVersion},
		catalog: NewFallbackCatalog(root),
	}
	block := BuildCrossRealmContext(idx, []string{uiFile, "desktop/app.go"}, "go test ./...")
	if block == "" || !strings.Contains(block, "## Wails Call Chain") {
		t.Fatalf("block = %q", block)
	}
}

func TestAutoBreakpointsResolvedSingleNodeFile(t *testing.T) {
	root := t.TempDir()
	g := NewGraph(root)
	uiFile := "desktop/frontend/src/components/Solo.tsx"
	ui := NewUIID(uiFile, "Solo")
	bridge := NewBridgeCallID(uiFile, 5, "Submit")
	gobind := NewGoBindID("desktop/app.go", "App.Submit")
	g.AddNode(&Node{ID: ui, Kind: KindUIComponent, Name: "Solo", File: uiFile})
	g.AddNode(&Node{ID: bridge, Kind: KindBridgeCall, Name: "app.Submit", File: uiFile, Line: 5})
	g.AddNode(&Node{ID: gobind, Kind: KindGoBind, Name: "App.Submit", File: "desktop/app.go"})
	g.AddEdge(ui, bridge, EdgeCalls)
	g.AddEdge(bridge, gobind, EdgeBridgeInvoke)
	g.RebuildIndexes()
	idx := &Index{
		root:    root,
		graph:   g,
		meta:    &Meta{IndexVersion: IndexVersion},
		catalog: NewFallbackCatalog(root),
	}
	bps, err := idx.AutoBreakpoints(context.Background(), []string{uiFile, "desktop/app.go"}, "Submit")
	if err != nil || len(bps) == 0 {
		t.Fatalf("bps=%v err=%v", bps, err)
	}
}

func TestAutoBreakpointsForwardBranchOnly(t *testing.T) {
	root := t.TempDir()
	g := NewGraph(root)
	hookFile := "desktop/frontend/src/lib/useX.ts"
	hook := NewHookID(hookFile, "useX")
	gobind := NewGoBindID("desktop/app.go", "App.Submit")
	g.AddNode(&Node{ID: hook, Kind: KindHook, Name: "useX", File: hookFile})
	g.AddNode(&Node{ID: gobind, Kind: KindGoBind, Name: "App.Submit", File: "desktop/app.go"})
	g.AddEdge(hook, gobind, EdgeBridgeInvoke)
	g.RebuildIndexes()
	idx := &Index{
		root:    root,
		graph:   g,
		meta:    &Meta{IndexVersion: IndexVersion},
		catalog: NewFallbackCatalog(root),
	}
	bps, err := idx.AutoBreakpoints(context.Background(), []string{hookFile, "desktop/app.go"}, "")
	if err != nil || len(bps) == 0 {
		t.Fatalf("bps=%v err=%v", bps, err)
	}
}

func TestBuildGraphEmptyRootRequired(t *testing.T) {
	if _, _, err := BuildGraph(BuildOptions{Root: ""}); err == nil {
		t.Fatal("expected empty root error")
	}
}

func TestOpenCallgraphEmptyRoot(t *testing.T) {
	if _, err := Open("  ", nil); err == nil {
		t.Fatal("expected empty root error")
	}
}

func TestCrossRealmChangeNilCatalogGuard(t *testing.T) {
	idx := &Index{root: t.TempDir(), catalog: nil}
	if crossRealmChange(idx, []string{"desktop/app.go", "desktop/frontend/src/x.ts"}) {
		t.Fatal("nil catalog should not count as cross-realm")
	}
}

func TestScanTSEventsOnLiteralAnonymousScope(t *testing.T) {
	rel := "desktop/frontend/src/lib/top_listen.ts"
	abs := filepath.Join(t.TempDir(), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	src := `EventsOn("agent:literal", () => {});`
	if err := os.WriteFile(abs, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, listens, _ := scanTSSourceFile(rel, abs, nil)
	found := false
	for _, l := range listens {
		if l.Channel == "agent:literal" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("listens=%v", listens)
	}
}

func TestScanTSMockRegionEndMarker(t *testing.T) {
	rel := "desktop/frontend/src/lib/mock_end.ts"
	abs := filepath.Join(t.TempDir(), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "// --- browser dev mock\nskip me\n// --- end mock\nexport function AfterMock() {}\n"
	if err := os.WriteFile(abs, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	symbols, _, _, _ := scanTSSourceFile(rel, abs, nil)
	if len(symbols) == 0 || symbols[0].Name != "AfterMock" {
		t.Fatalf("symbols=%v", symbols)
	}
}

func TestTSBraceBalancedEscapedQuote(t *testing.T) {
	if !tsBraceBalanced("const x = \"a \\\" { \";\n") {
		t.Fatal("expected balanced braces with escaped quote")
	}
	if tsBraceBalanced("}{") {
		t.Fatal("expected unbalanced")
	}
}

func TestExtractBridgeMethodsWindowAndCase(t *testing.T) {
	var warns []ParseWarning
	if got := extractBridgeMethods("window.go.main.App.Submit()", &warns, "f.ts", 1); len(got) == 0 {
		t.Fatal("expected window bridge method")
	}
	extractBridgeMethods("app.submit()", &warns, "f.ts", 2)
	foundCase := false
	for _, w := range warns {
		if strings.Contains(w.Message, "method_name_case_mismatch") {
			foundCase = true
			break
		}
	}
	if !foundCase {
		t.Fatalf("warns=%v", warns)
	}
}

func TestGraphAddEdgeDedupes(t *testing.T) {
	g := NewGraph(t.TempDir())
	a, b := NewHookID("a.ts", "A"), NewBridgeCallID("a.ts", 1, "Submit")
	g.AddNode(&Node{ID: a, Kind: KindHook, Name: "A", File: "a.ts"})
	g.AddNode(&Node{ID: b, Kind: KindBridgeCall, Name: "app.Submit", File: "a.ts", Line: 1})
	g.AddEdge(a, b, EdgeCalls)
	before := g.EdgeCount()
	g.AddEdge(a, b, EdgeCalls)
	if g.EdgeCount() != before {
		t.Fatalf("edge count = %d want %d", g.EdgeCount(), before)
	}
}

func TestParseAppBindingsMethodFormOnly(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "desktop", "frontend", "src", "lib")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "export interface AppBindings {\n  Submit(arg: string)\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "bridge.ts"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	methods, err := ParseAppBindings(root)
	if err != nil || !methods["Submit"] {
		t.Fatalf("methods=%v err=%v", methods, err)
	}
}

func TestCgTraceBackwardIncludeEventsOption(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	toolDef := cgTraceBackwardTool{idx: idx}
	out, err := toolDef.Execute(context.Background(), json.RawMessage(`{"go_method":"Notify","include_events":true,"max_paths":2}`))
	if err != nil || !strings.Contains(out, "Trace backward") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestCgFindBridgeToolSymbolForward(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	toolDef := cgFindBridgeTool{idx: idx}
	out, err := toolDef.Execute(context.Background(), json.RawMessage(`{"frontend":"desktop/frontend/src/lib/useSubmit.ts#useSubmit","go_method":"Submit"}`))
	if err != nil || !strings.Contains(out, "Find bridge") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestBuildGraphHookUsedByEdge(t *testing.T) {
	g := NewGraph(t.TempDir())
	ui := NewUIID("panel.tsx", "Panel")
	hook := NewHookID("hooks.ts", "usePanel")
	g.AddNode(&Node{ID: ui, Kind: KindUIComponent, Name: "Panel", File: "panel.tsx"})
	g.AddNode(&Node{ID: hook, Kind: KindHook, Name: "usePanel", File: "hooks.ts"})
	for _, c := range []TSCall{{From: ui, To: hook, Kind: EdgeCalls}} {
		kind := c.Kind
		if kind == "" {
			kind = EdgeCalls
		}
		g.AddEdge(c.From, c.To, kind)
		toNode, okTo := g.Node(c.To)
		fromNode, okFrom := g.Node(c.From)
		if okTo && okFrom && toNode.Kind == KindHook && fromNode.Kind == KindUIComponent {
			g.AddEdge(c.To, c.From, EdgeHookUsedBy)
		}
	}
	found := false
	for _, e := range g.edges {
		if e.Kind == EdgeHookUsedBy {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected EdgeHookUsedBy edge")
	}
}

func TestOpenProjectDirFailure(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "block")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(blocker, nil); err == nil {
		t.Fatal("expected ProjectDir failure when workspace is a file")
	}
}
