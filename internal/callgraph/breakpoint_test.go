package callgraph

import (
	"context"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestBreakpointsFromPath(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	paths := TraceBackward(g, g.MethodMap["Submit"], DefaultTraceOptions())
	if len(paths) == 0 {
		t.Fatal("expected paths")
	}
	bps := BreakpointsFromPath(paths[0])
	if len(bps) == 0 {
		t.Fatal("expected breakpoints")
	}
	layers := map[string]bool{}
	for _, bp := range bps {
		layers[bp.Layer] = true
		if bp.File == "" || bp.Symbol == "" || bp.Reason == "" {
			t.Fatalf("incomplete bp: %+v", bp)
		}
	}
	if !layers["go"] {
		t.Fatalf("missing go layer: %+v", bps)
	}
}

func TestSuggestBreakpointsDedupes(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	paths := TraceBackward(g, g.MethodMap["Submit"], DefaultTraceOptions())
	bps := SuggestBreakpoints(append(paths, paths...))
	if len(bps) == 0 {
		t.Fatal("expected breakpoints")
	}
}

func TestAutoBreakpointsCrossRealm(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	bps, err := idx.AutoBreakpoints(context.Background(), []string{
		"desktop/app.go",
		"desktop/frontend/src/lib/useSubmit.ts",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(bps) == 0 {
		t.Fatal("expected auto breakpoints for cross-realm change")
	}
}

func TestAutoBreakpointsSkipsSingleRealm(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = idx.EnsureReady(context.Background())
	bps, err := idx.AutoBreakpoints(context.Background(), []string{"desktop/app.go"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(bps) != 0 {
		t.Fatalf("got %v", bps)
	}
}

func TestBreakpointsForQuery(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	bps, paths, err := idx.BreakpointsForQuery(context.Background(),
		"desktop/frontend/src/lib/useSubmit.ts", "useSubmit", "Submit")
	if err != nil {
		t.Fatal(err)
	}
	if len(bps) == 0 || len(paths) == 0 {
		t.Fatalf("bps=%d paths=%d", len(bps), len(paths))
	}
}

func TestFormatBreakpointContext(t *testing.T) {
	out := FormatBreakpointContext([]Breakpoint{
		{File: "desktop/app.go", Line: 10, Symbol: "App.Submit", Layer: "go", Reason: "bind"},
	})
	if !strings.Contains(out, "## Debug Breakpoints") || !strings.Contains(out, "App.Submit") {
		t.Fatalf("out=%q", out)
	}
}

func TestBreakpointLayerKinds(t *testing.T) {
	layer, reason := breakpointLayer(NodeSnapshot{Kind: KindBridgeCall, Name: "app.Submit"}, EdgeBridgeInvoke, "rpc")
	if layer != "bridge" || reason == "" {
		t.Fatalf("got %q %q", layer, reason)
	}
}

func TestBuildCrossRealmContextIncludesBreakpoints(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = idx.EnsureReady(context.Background())
	block := BuildCrossRealmContext(idx, []string{
		"desktop/app.go",
		"desktop/frontend/src/lib/useSubmit.ts",
	}, "go test ./...")
	if block == "" {
		t.Fatal("expected block")
	}
	if !strings.Contains(block, "## Debug Breakpoints") {
		t.Fatalf("block=%q", block)
	}
}

func TestBidirectionalBridge(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	id, err := ResolveNodeID(g, "desktop/frontend/src/lib/useSubmit.ts", "useSubmit")
	if err != nil {
		t.Fatal(err)
	}
	paths, err := FindBridgePath(g, id, "Submit")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("expected bridge path")
	}
}

func TestBreakpointsTool(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	reg := newToolRegistry(t)
	RegisterTools(reg, idx)
	tool, ok := reg.Get("callgraph_breakpoints")
	if !ok {
		t.Fatal("missing tool")
	}
	out, err := tool.Execute(context.Background(), []byte(`{"from":"desktop/frontend/src/lib/useSubmit.ts#useSubmit","go_method":"Submit"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Debug Breakpoints") {
		t.Fatalf("out=%q", out)
	}
}

func TestBreakpointsToolChangedPaths(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = idx.EnsureReady(context.Background())
	reg := newToolRegistry(t)
	RegisterTools(reg, idx)
	tool, _ := reg.Get("callgraph_breakpoints")
	out, err := tool.Execute(context.Background(), []byte(`{"changed_paths":["desktop/app.go","desktop/frontend/src/lib/useSubmit.ts"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Debug Breakpoints") {
		t.Fatalf("out=%q", out)
	}
}

func newToolRegistry(t *testing.T) *tool.Registry {
	t.Helper()
	return tool.NewRegistry()
}
