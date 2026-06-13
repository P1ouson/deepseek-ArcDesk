package callgraph

import (
	"context"
	"strings"
	"testing"
)

func TestBreakpointLayerAllKinds(t *testing.T) {
	cases := []struct {
		kind NodeKind
		edge EdgeKind
		path string
		want string
	}{
		{KindUIHandler, EdgeCalls, "rpc", "ui"},
		{KindHook, EdgeCalls, "rpc", "ui"},
		{KindBridgeCall, EdgeBridgeInvoke, "rpc", "bridge"},
		{KindTSFunction, EdgeBridgeInvoke, "rpc", "bridge"},
		{KindGoBind, EdgeGoCalls, "rpc", "go"},
		{KindEventListen, EdgeEventDelivers, "event", "event"},
		{KindEventEmit, EdgeEmits, "event", "event"},
		{KindGoInternal, EdgeCalls, "rpc", ""},
	}
	for _, tc := range cases {
		layer, _ := breakpointLayer(NodeSnapshot{Kind: tc.kind, Name: "x"}, tc.edge, tc.path)
		if layer != tc.want {
			t.Fatalf("%+v: got layer %q want %q", tc, layer, tc.want)
		}
	}
}

func TestAutoBreakpointsNilIndex(t *testing.T) {
	var idx *Index
	if _, err := idx.AutoBreakpoints(context.Background(), nil, ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestAutoBreakpointsResolveErrorUsesGoFile(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	bps, err := idx.AutoBreakpoints(context.Background(), []string{"desktop/app.go", "desktop/frontend/src/lib/useSubmit.ts"}, "Submit")
	if err != nil {
		t.Fatal(err)
	}
	if len(bps) == 0 {
		t.Fatal("expected breakpoints")
	}
}

func TestBreakpointsForQueryGoMethodOnly(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	bps, paths, err := idx.BreakpointsForQuery(context.Background(), "", "", "Submit")
	if err != nil {
		t.Fatal(err)
	}
	if len(bps) == 0 || len(paths) == 0 {
		t.Fatal("expected results")
	}
}

func TestBreakpointsForQueryMissingMethod(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	_, _, err := idx.BreakpointsForQuery(context.Background(), "", "", "NoSuchMethod")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBreakpointsForQueryNoMatch(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	_, _, err := idx.BreakpointsForQuery(context.Background(), "desktop/frontend/src/lib/bridge.ts", "missingSymbol", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatBreakpointContextEmpty(t *testing.T) {
	if got := FormatBreakpointContext(nil); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatBreakpointContextCaps(t *testing.T) {
	var bps []Breakpoint
	for i := 0; i < 10; i++ {
		bps = append(bps, Breakpoint{File: "f.go", Symbol: "S", Layer: "go", Reason: "r"})
	}
	out := FormatBreakpointContext(bps)
	if !strings.Contains(out, "…") {
		t.Fatalf("out=%q", out)
	}
}

func TestBidirectionalBridgeNil(t *testing.T) {
	if got := bidirectionalBridge(nil, "a", "b", DefaultTraceOptions()); got != nil {
		t.Fatal("expected nil")
	}
}

func TestMergeBridgePaths(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	fwd := []pathStep{{node: g.MethodMap["Submit"]}, {node: "x"}}
	bwd := []pathStep{{node: "y"}, {node: "x"}}
	p := mergeBridgePaths(g, fwd, bwd)
	if len(p.Segments) == 0 {
		t.Fatal("expected merged path")
	}
}

func TestCgBreakpointsToolEmptyChangedPaths(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	tool := cgBreakpointsTool{idx: idx}
	out, err := tool.Execute(context.Background(), []byte(`{"changed_paths":["desktop/app.go"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "No cross-realm") {
		t.Fatalf("out=%q", out)
	}
}

func TestCgBreakpointsToolMetadata(t *testing.T) {
	tool := cgBreakpointsTool{}
	if tool.Name() != "callgraph_breakpoints" || tool.Description() == "" || !tool.ReadOnly() {
		t.Fatal("metadata")
	}
}

func TestSuggestBreakpointsEmptyPath(t *testing.T) {
	if got := BreakpointsFromPath(CallPath{}); got != nil {
		t.Fatalf("got %v", got)
	}
}

func TestAutoBreakpointsWithGoMethod(t *testing.T) {
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

func TestBridgeImpactAdapterNil(t *testing.T) {
	var nilIdx *Index
	if nilIdx.BridgeImpactAnalyzer().Available() {
		t.Fatal("nil index should not be available")
	}
}

func TestBidirectionalBridgeOnProject(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	frontend, err := ResolveNodeID(g, "desktop/frontend/src/lib/useSubmit.ts", "useSubmit")
	if err != nil {
		t.Fatal(err)
	}
	gobind := g.MethodMap["Submit"]
	opts := DefaultTraceOptions()
	got := bidirectionalBridge(g, frontend, gobind, opts)
	if len(got) == 0 {
		t.Fatal("expected bidirectional path")
	}
}

func TestBuildCallPathWithInternals(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	steps := []pathStep{{node: g.MethodMap["Submit"]}}
	p := buildCallPathWithInternals(g, steps, SymbolRef{Name: "helper", File: "desktop/app.go", Line: 1})
	if len(p.Segments) < 2 {
		t.Fatal("expected extended path")
	}
}

func TestCgBreakpointsToolInvalidQuery(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	_ = idx.EnsureReady(context.Background())
	tool := cgBreakpointsTool{idx: idx}
	if _, err := tool.Execute(context.Background(), []byte(`{}`)); err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestFindBridgePathMissingMethod(t *testing.T) {
	g := &CallGraph{MethodMap: map[string]NodeID{}}
	_, err := FindBridgePath(g, "frontend", "Missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildCrossRealmContextEarlyExit(t *testing.T) {
	if got := BuildCrossRealmContext(nil, []string{"a.ts"}, "go test ./..."); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := BuildCrossRealmContext(&Index{}, []string{"a.ts"}, "git status"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatCrossRealmContextEventPath(t *testing.T) {
	out := FormatCrossRealmContext([]CallPath{{
		PathKind:     "event",
		EventChannel: "agent:event",
		Segments: []PathSegment{
			{Node: NodeSnapshot{Kind: KindEventEmit, Name: "agent:event"}},
			{Node: NodeSnapshot{Kind: KindEventListen, Name: "agent:event"}, Edge: EdgeEventDelivers},
		},
	}}, "Notify")
	if !strings.Contains(out, "Event") {
		t.Fatalf("out=%q", out)
	}
}

func TestBridgeImpactAdapterAffectedUINil(t *testing.T) {
	a := bridgeImpactAdapter{}
	if _, err := a.AffectedUI("Submit"); err == nil {
		t.Fatal("expected error")
	}
	if a.Available() {
		t.Fatal("nil idx not available")
	}
}

func TestIsVerifyCommandEdgeCases(t *testing.T) {
	if IsVerifyCommand("") || IsVerifyCommand("git status") {
		t.Fatal("expected false")
	}
	if !IsVerifyCommand("pnpm build") {
		t.Fatal("expected true")
	}
}
