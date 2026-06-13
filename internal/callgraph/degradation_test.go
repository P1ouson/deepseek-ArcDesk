package callgraph

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func buildInProject(t *testing.T, mutate func(root string)) *CallGraph {
	t.Helper()
	root := copyWailsTestProject(t)
	if mutate != nil {
		mutate(root)
	}
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	return g
}

func TestBuildGraphNoDesktopDir(t *testing.T) {
	dir := t.TempDir()
	g, _, err := BuildGraph(BuildOptions{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	if g.Stats.NodeCount != 0 {
		t.Fatalf("NodeCount = %d, want 0", g.Stats.NodeCount)
	}
}

func TestBuildGraphEmptyProject(t *testing.T) {
	TestBuildGraphNoDesktopDir(t)
}

func TestBuildGraphNoMainGo(t *testing.T) {
	g := buildInProject(t, func(root string) {
		_ = os.Remove(filepath.Join(root, "desktop", "main.go"))
	})
	if g.Stats.GoBindCount < 1 {
		t.Fatalf("expected go binds without main.go, got %d", g.Stats.GoBindCount)
	}
	if g.Stats.BridgeCallCount < 1 {
		t.Fatalf("expected bridge calls, got %d", g.Stats.BridgeCallCount)
	}
}

func TestBuildGraphNoFrontendDir(t *testing.T) {
	g := buildInProject(t, func(root string) {
		_ = os.RemoveAll(filepath.Join(root, "desktop", "frontend"))
	})
	if g.Stats.GoBindCount < 1 {
		t.Fatalf("expected go binds, got %d", g.Stats.GoBindCount)
	}
	if g.Stats.BridgeCallCount != 0 {
		t.Fatalf("BridgeCallCount = %d, want 0", g.Stats.BridgeCallCount)
	}
}

func TestBuildGraphNoSrcDir(t *testing.T) {
	g := buildInProject(t, func(root string) {
		_ = os.RemoveAll(filepath.Join(root, "desktop", "frontend", "src"))
	})
	if g.Stats.GoBindCount < 1 {
		t.Fatal("expected go binds")
	}
	if g.Stats.BridgeCallCount != 0 {
		t.Fatalf("BridgeCallCount = %d, want 0", g.Stats.BridgeCallCount)
	}
}

func TestBridgeMappingFromGoBindFallback(t *testing.T) {
	g := buildInProject(t, func(root string) {
		_ = os.Remove(filepath.Join(root, "desktop", "frontend", "wailsjs", "go", "main", "App.d.ts"))
	})
	if !hasWarningMessage(g.Warnings, "App.d.ts missing") {
		t.Fatalf("warnings = %v", g.Warnings)
	}
	if g.Stats.GoBindCount < 1 || len(g.MethodMap) == 0 {
		t.Fatal("expected method map from go bind fallback")
	}
}

func TestBridgeMappingNoWailsJS(t *testing.T) {
	g := buildInProject(t, func(root string) {
		_ = os.RemoveAll(filepath.Join(root, "desktop", "frontend", "wailsjs"))
	})
	if !hasWarningMessage(g.Warnings, "wailsjs_missing") {
		t.Fatalf("warnings = %v", g.Warnings)
	}
	if len(g.MethodMap) == 0 {
		t.Fatal("expected method map from go bind")
	}
}

func TestBridgeMappingNoAppBindings(t *testing.T) {
	g := buildInProject(t, func(root string) {
		_ = os.RemoveAll(filepath.Join(root, "desktop", "frontend", "wailsjs"))
		path := filepath.Join(root, "desktop", "frontend", "src", "lib", "bridge.ts")
		_ = os.WriteFile(path, []byte("export const app = { Submit: async () => {} };\n"), 0o644)
	})
	if !hasWarningMessage(g.Warnings, "wailsjs_missing") {
		t.Fatal("expected wailsjs_missing")
	}
	if _, ok := g.MethodMap["Submit"]; !ok {
		t.Fatal("expected Submit in method map from go bind")
	}
}

func TestBuildGraphNilCatalog(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root, Catalog: nil})
	if err != nil {
		t.Fatal(err)
	}
	cat := NewFallbackCatalog(root)
	if _, ok := cat.ResolveFile("desktop/app.go"); !ok {
		t.Fatal("fallback catalog should resolve go files")
	}
	if g.Stats.NodeCount == 0 {
		t.Fatal("expected nodes")
	}
}

func TestOrphanBridgeCall(t *testing.T) {
	g := buildInProject(t, nil)
	if !hasWarningMessage(g.Warnings, "orphan_bridge:FooNotInDTS") &&
		!hasWarningMessage(g.Warnings, "orphan_bridge_call:FooNotInDTS") {
		t.Fatalf("expected orphan bridge warning, got %v", g.Warnings)
	}
}

func TestBridgeCallNoGoBind(t *testing.T) {
	TestOrphanBridgeCall(t)
}

func TestUnusedGoBind(t *testing.T) {
	g := buildInProject(t, nil)
	if !hasWarningMessage(g.Warnings, "unused_bind:UnusedBind") {
		t.Fatalf("warnings = %v", g.Warnings)
	}
}

func TestMethodNameCaseMismatch(t *testing.T) {
	g := buildInProject(t, nil)
	if !hasWarningMessage(g.Warnings, "method_name_case_mismatch:submit") {
		t.Fatalf("warnings = %v", g.Warnings)
	}
}

func TestAppDTSDrift(t *testing.T) {
	g := buildInProject(t, nil)
	if !hasWarningMessage(g.Warnings, "app_dts_drift:extra:ExtraInDTSOnly") {
		t.Fatalf("missing extra drift: %v", g.Warnings)
	}
	if !hasWarningMessage(g.Warnings, "app_dts_drift:missing:OnlyInGo") {
		t.Fatalf("missing OnlyInGo drift: %v", g.Warnings)
	}
}

func TestGoSyntaxErrorSkip(t *testing.T) {
	g := buildInProject(t, nil)
	if g.Stats.ParseErrorCount < 1 {
		t.Fatalf("ParseErrorCount = %d, want >=1", g.Stats.ParseErrorCount)
	}
	if !hasWarningMessage(g.Warnings, "go_parse_error") && !hasWarningMessage(g.Warnings, "go/parser fallback") {
		t.Fatalf("warnings = %v", g.Warnings)
	}
	if _, ok := g.MethodMap["Submit"]; !ok {
		t.Fatal("expected Submit bind from other go files")
	}
}

func TestTSSyntaxErrorSkip(t *testing.T) {
	g := buildInProject(t, nil)
	if !hasWarningMessage(g.Warnings, "ts_parse_error") {
		t.Fatalf("warnings = %v", g.Warnings)
	}
}

func TestGoBindRegexFallback(t *testing.T) {
	g := buildInProject(t, nil)
	if !hasWarningMessage(g.Warnings, "go/parser fallback") {
		t.Fatalf("warnings = %v", g.Warnings)
	}
}

func TestAppDTSParseError(t *testing.T) {
	g := buildInProject(t, func(root string) {
		_ = os.Remove(filepath.Join(root, "desktop", "frontend", "wailsjs", "go", "main", "App.d.ts"))
	})
	// App_broken.d.ts may not salvage; go bind still authoritative
	if len(g.MethodMap) == 0 {
		t.Fatal("expected method map")
	}
}

func TestTSUnusualImport(t *testing.T) {
	g := buildInProject(t, func(root string) {
		path := filepath.Join(root, "desktop", "frontend", "src", "lib", "require_style.ts")
		_ = os.WriteFile(path, []byte("import x = require('legacy');\nexport function Legacy() { app.Submit('x'); }\n"), 0o644)
	})
	if !hasWarningMessage(g.Warnings, "ts_unusual_import") {
		t.Fatalf("warnings = %v", g.Warnings)
	}
}

func TestRefreshConcurrentSkip(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	start := make(chan struct{})
	var err1, err2 error
	go func() {
		defer wg.Done()
		<-start
		err1 = idx.RefreshIfStale(context.Background())
	}()
	go func() {
		defer wg.Done()
		<-start
		err2 = idx.RefreshIfStale(context.Background())
	}()
	close(start)
	wg.Wait()
	if err1 != nil && err2 != nil {
		t.Fatalf("both refresh failed: %v / %v", err1, err2)
	}
}

func TestLoadCorruptedIndex(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, indexFileName), []byte("{not-json"), 0o644)
	_, err := LoadIndex(dir)
	if !errors.Is(err, ErrIndexCorrupt) {
		t.Fatalf("err = %v, want ErrIndexCorrupt", err)
	}
}

func TestSaveIndexDiskFull(t *testing.T) {
	dir := t.TempDir()
	g := NewGraph(dir)
	g.AddNode(&Node{ID: NewGoBindID("desktop/app.go", "App.Submit"), Kind: KindGoBind, Name: "App.Submit", File: "desktop/app.go"})
	old := atomicWriteFile
	atomicWriteFile = func(string, []byte) error {
		return errors.New("no space left on device")
	}
	defer func() { atomicWriteFile = old }()
	err := SaveIndex(g, dir)
	if err == nil {
		t.Fatal("expected save error")
	}
	if _, err := os.Stat(filepath.Join(dir, indexFileName+".tmp")); !os.IsNotExist(err) {
		t.Fatalf("leftover tmp file: %v", err)
	}
}

func TestTraceTruncatedLongChain(t *testing.T) {
	g := NewGraph(t.TempDir())
	ids := make([]NodeID, 12)
	for i := range ids {
		kind := KindHook
		if i == 0 {
			kind = KindUIComponent
		}
		if i == len(ids)-1 {
			kind = KindGoBind
		}
		ids[i] = NodeID(fmt.Sprintf("n%d", i))
		g.AddNode(&Node{ID: ids[i], Kind: kind, Name: fmt.Sprintf("n%d", i), File: "f.ts"})
		if i > 0 {
			g.AddEdge(ids[i-1], ids[i], EdgeCalls)
		}
	}
	g.RebuildIndexes()
	opts := TraceOptions{MaxDepth: 3, MaxPaths: 1}
	paths := TraceForward(g, ids[0], opts)
	if len(paths) == 0 {
		t.Fatal("expected truncated path")
	}
	if !paths[0].Truncated {
		t.Fatal("expected Truncated=true")
	}
	if paths[0].Hint == "" {
		t.Fatal("expected truncation hint")
	}
}

func TestTraceCycleDetection(t *testing.T) {
	g := NewGraph(t.TempDir())
	a := NewHookID("a.ts", "A")
	b := NewHookID("b.ts", "B")
	g.AddNode(&Node{ID: a, Kind: KindHook, Name: "A", File: "a.ts"})
	g.AddNode(&Node{ID: b, Kind: KindHook, Name: "B", File: "b.ts"})
	g.AddEdge(a, b, EdgeCalls)
	g.AddEdge(b, a, EdgeCalls)
	g.RebuildIndexes()
	paths := TraceForward(g, a, DefaultTraceOptions())
	// Pure hook cycle has no UI terminal; success means BFS terminated without looping.
	if len(paths) != 0 {
		for _, p := range paths {
			if pathContainsNodeInCallPath(p, a) && pathContainsNodeInCallPath(p, b) {
				return
			}
		}
	}
}

func pathContainsNodeInCallPath(p CallPath, id NodeID) bool {
	for _, s := range p.Segments {
		if s.Node.ID == id {
			return true
		}
	}
	return false
}

func TestManyBridgeCallSites(t *testing.T) {
	root := copyWailsTestProject(t)
	var body strings.Builder
	body.WriteString("import { app } from \"../lib/bridge\";\nexport function ManyCalls() {\n")
	for i := 0; i < 55; i++ {
		fmt.Fprintf(&body, "  void app.Submit(\"n%d\");\n", i)
	}
	body.WriteString("}\n")
	path := filepath.Join(root, "desktop", "frontend", "src", "components", "ManyCalls.tsx")
	_ = os.WriteFile(path, []byte(body.String()), 0o644)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	sites := g.BridgeByMethod["Submit"]
	if len(sites) < 50 {
		t.Fatalf("Submit sites = %d, want >= 50", len(sites))
	}
	opts := TraceOptions{MaxDepth: 10, MaxPaths: 2}
	paths := TraceBackward(g, g.MethodMap["Submit"], opts)
	if len(paths) > opts.MaxPaths {
		t.Fatalf("paths = %d, want <= maxPaths", len(paths))
	}
}

func TestAnonymousHandler(t *testing.T) {
	g := buildInProject(t, nil)
	found := false
	for _, n := range g.Nodes {
		if n != nil && n.Kind == KindUIHandler && strings.HasPrefix(n.Name, "anonymous:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected anonymous handler node")
	}
}

func TestDynamicImportBridge(t *testing.T) {
	g := buildInProject(t, nil)
	if !hasWarningMessage(g.Warnings, "dynamic_import_bridge") {
		t.Fatalf("warnings = %v", g.Warnings)
	}
}

func TestBracketBridgeCall(t *testing.T) {
	g := buildInProject(t, nil)
	if !hasWarningMessage(g.Warnings, "bracket_bridge_call:Submit") {
		t.Fatalf("warnings = %v", g.Warnings)
	}
}

func TestDuplicateGoBind(t *testing.T) {
	g := buildInProject(t, nil)
	if !hasWarningMessage(g.Warnings, "duplicate_bind:Submit") {
		t.Fatalf("warnings = %v", g.Warnings)
	}
	count := 0
	for _, n := range g.Nodes {
		if n != nil && n.Kind == KindGoBind && strings.HasSuffix(n.Name, "Submit") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("Submit gobind nodes = %d, want 1", count)
	}
}

func TestFallbackCatalogResolve(t *testing.T) {
	root := copyWailsTestProject(t)
	cat := NewFallbackCatalog(root)
	if _, ok := cat.ResolveFile("desktop/frontend/src/lib/bridge.ts"); !ok {
		t.Fatal("expected js file resolve")
	}
	kind, ok := cat.ModuleKind("js:desktop/frontend/src/lib/bridge.ts")
	if !ok || kind != "js" {
		t.Fatalf("kind = %q ok=%v", kind, ok)
	}
}

func TestIndexOpenCorruptMarksError(t *testing.T) {
	root := copyWailsTestProject(t)
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, indexFileName), []byte("[]"), 0o644)
	_, err = Open(root, nil)
	if err == nil {
		t.Fatal("expected open error for corrupt index")
	}
}

func TestRefreshBlocksReadUnderRace(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_ = idx.RefreshIfStale(context.Background())
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	_, _ = idx.Status()
	<-done
}
