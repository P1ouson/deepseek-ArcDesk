package callgraph

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/dependency"
)

func buildEventProject(t *testing.T) *CallGraph {
	t.Helper()
	return buildInProject(t, nil)
}

func TestEventEmitLiteral(t *testing.T) {
	g := buildEventProject(t)
	if g.Stats.EventEmitCount == 0 {
		t.Fatalf("EventEmitCount = 0, want > 0")
	}
	found := false
	for _, n := range g.Nodes {
		if n != nil && n.Kind == KindEventEmit && n.Name == "agent:event" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected agent:event emit node")
	}
}

func TestEventEmitVariableWarning(t *testing.T) {
	g := buildEventProject(t)
	if !hasWarningMessage(g.Warnings, "event_emit_variable_channel") {
		t.Fatalf("warnings = %v", g.Warnings)
	}
}

func TestEventListenDirect(t *testing.T) {
	g := buildEventProject(t)
	found := false
	for _, n := range g.Nodes {
		if n != nil && n.Kind == KindEventListen && n.Name == "agent:event" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected agent:event listen node")
	}
}

func TestEventListenWrapped(t *testing.T) {
	g := buildEventProject(t)
	foundHandler := false
	foundListen := false
	for _, n := range g.Nodes {
		if n == nil {
			continue
		}
		if n.Kind == KindUIHandler && n.Name == "onAgentEvent" {
			foundHandler = true
		}
		if n.Kind == KindEventListen && n.Name == "agent:event" && strings.Contains(n.File, "events_bridge.ts") {
			foundListen = true
		}
	}
	if !foundHandler || !foundListen {
		t.Fatalf("handler=%v listen=%v", foundHandler, foundListen)
	}
}

func TestEventDeliversMatch(t *testing.T) {
	g := buildEventProject(t)
	if g.Stats.EventDeliverCount == 0 {
		t.Fatalf("EventDeliverCount = 0, want > 0")
	}
}

func TestEventDeliversNoMatch(t *testing.T) {
	g := NewGraph(t.TempDir())
	emit := NewEventEmitID("desktop/app.go", 1, "missing:channel")
	listen := NewEventListenID("desktop/frontend/src/App.tsx", 2, "other:channel")
	g.AddNode(&Node{ID: emit, Kind: KindEventEmit, Name: "missing:channel", File: "desktop/app.go", Line: 1})
	g.AddNode(&Node{ID: listen, Kind: KindEventListen, Name: "other:channel", File: "desktop/frontend/src/App.tsx", Line: 2})
	count := LinkEventDelivers(g)
	if count != 0 {
		t.Fatalf("EventDeliverCount = %d, want 0", count)
	}
}

func TestMultipleEventsEmit(t *testing.T) {
	g := buildEventProject(t)
	count := 0
	for _, n := range g.Nodes {
		if n != nil && n.Kind == KindEventEmit && strings.Contains(n.File, "app_events.go") && n.Name == "agent:ready" || n != nil && n.Kind == KindEventEmit && n.Name == "terminal:output" {
			count++
		}
	}
	// MultiEmit creates two distinct emit nodes.
	multi := 0
	for _, n := range g.Nodes {
		if n != nil && n.Kind == KindEventEmit && strings.Contains(n.File, "app_events.go") {
			if n.Name == "agent:ready" || n.Name == "terminal:output" {
				multi++
			}
		}
	}
	if multi < 2 {
		t.Fatalf("MultiEmit nodes = %d, want >= 2", multi)
	}
	_ = count
}

func TestTraceBackwardWithEvent(t *testing.T) {
	g := buildEventProject(t)
	gobind := g.MethodMap["Notify"]
	if gobind == "" {
		t.Fatal("Notify bind missing")
	}
	opts := DefaultTraceOptions()
	opts.IncludeEvents = true
	paths := TraceBackward(g, gobind, opts)
	foundEvent := false
	for _, p := range paths {
		if p.EventChannel == "agent:event" || p.PathKind == "event" {
			foundEvent = true
		}
		for _, seg := range p.Segments {
			if seg.Node.Kind == KindEventListen || seg.Node.Kind == KindEventEmit {
				foundEvent = true
			}
		}
	}
	if !foundEvent {
		t.Fatalf("expected event path, got %d paths", len(paths))
	}
}

func TestFormatEventPath(t *testing.T) {
	g := buildEventProject(t)
	paths := TraceBackward(g, g.MethodMap["Notify"], DefaultTraceOptions())
	block := FormatCrossRealmContext(paths, "Notify")
	if !strings.Contains(block, "Event") {
		t.Fatalf("format = %q", block)
	}
	if !strings.Contains(block, "agent:event") {
		t.Fatalf("format = %q", block)
	}
}

func TestCrossRealmImpact(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.RefreshIfStale(context.Background()); err != nil {
		t.Fatal(err)
	}
	ui, err := idx.CrossRealmImpact("Submit")
	if err != nil {
		t.Fatal(err)
	}
	if len(ui) == 0 {
		t.Fatal("expected UI nodes for Submit")
	}
}

func TestCrossRealmImpactWithEvent(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.RefreshIfStale(context.Background()); err != nil {
		t.Fatal(err)
	}
	ui, err := idx.CrossRealmImpact("Notify")
	if err != nil {
		t.Fatal(err)
	}
	if len(ui) == 0 {
		t.Fatal("expected UI nodes via event path for Notify")
	}
}

func TestCrossRealmImpactNotFound(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.RefreshIfStale(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = idx.CrossRealmImpact("NoSuchMethod")
	if !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestCrossRealmAnalyzerUnavailable(t *testing.T) {
	g := dependency.NewGraph(t.TempDir())
	id := dependency.NewGoID("example.com/a")
	g.AddNode(&dependency.Node{ID: id, Kind: dependency.KindInternalGo, Name: "a", Meta: dependency.NodeMeta{BridgeMethod: "Submit"}})
	g.Impact[id] = dependency.ImpactLayers{}
	res, err := dependency.AffectedByWithAnalyzer(g, id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CrossRealm) != 0 {
		t.Fatalf("CrossRealm = %+v", res.CrossRealm)
	}
}

func TestTraceForwardWithGoInternal(t *testing.T) {
	g := buildEventProject(t)
	from, err := ResolveNodeID(g, "desktop/frontend/src/lib/useSubmit.ts", "useSubmit")
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultTraceOptions()
	opts.IncludeGoInternal = true
	opts.SymbolQuery = MockSymbolQuery{
		OK: true,
		Results: []SymbolRef{{Name: "doSubmit", File: "desktop/app.go", Line: 18, Kind: "method"}},
	}
	paths := TraceForward(g, from, opts)
	found := false
	for _, p := range paths {
		for _, seg := range p.Segments {
			if seg.Node.Kind == KindGoInternal && seg.Node.Name == "doSubmit" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected go_internal extension in forward trace")
	}
}

func TestTraceForwardNoSymbolQuery(t *testing.T) {
	g := buildEventProject(t)
	from, err := ResolveNodeID(g, "desktop/frontend/src/lib/useSubmit.ts", "useSubmit")
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultTraceOptions()
	opts.IncludeGoInternal = true
	paths := TraceForward(g, from, opts)
	for _, p := range paths {
		for _, seg := range p.Segments {
			if seg.Node.Kind == KindGoInternal && seg.Node.Name == "doSubmit" {
				t.Fatal("unexpected go_internal without symbol query")
			}
		}
	}
	if len(paths) == 0 {
		t.Fatal("expected forward paths")
	}
}

func TestSymbolQueryTimeout(t *testing.T) {
	q := MockSymbolQuery{
		OK:    true,
		Delay: 200 * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := q.Callees(ctx, "App.Submit", 3)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestDependencyAffectedByCrossRealm(t *testing.T) {
	g := dependency.NewGraph(t.TempDir())
	id := dependency.NewGoID("example.com/desktop")
	g.AddNode(&dependency.Node{
		ID:   id,
		Kind: dependency.KindInternalGo,
		Name: "desktop",
		Meta: dependency.NodeMeta{BridgeMethod: "Submit"},
	})
	g.Impact[id] = dependency.ImpactLayers{}
	mock := mockBridgeAnalyzer{entries: []dependency.CrossRealmImpactEntry{
		{ID: "ui:Composer", Name: "Composer", Kind: "ui_component"},
	}}
	res, err := dependency.AffectedByWithAnalyzer(g, id, mock)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CrossRealm) != 1 || res.CrossRealm[0].Name != "Composer" {
		t.Fatalf("CrossRealm = %+v", res.CrossRealm)
	}
}

type mockBridgeAnalyzer struct {
	entries []dependency.CrossRealmImpactEntry
}

func (m mockBridgeAnalyzer) Available() bool { return true }

func (m mockBridgeAnalyzer) AffectedUI(string) ([]dependency.CrossRealmImpactEntry, error) {
	return m.entries, nil
}

func TestOldIndexLoadsWithoutEventFields(t *testing.T) {
	root := copyWailsTestProject(t)
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(dir)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate v1 index without event node kinds.
	for id, n := range g.Nodes {
		if n != nil && (n.Kind == KindEventEmit || n.Kind == KindEventListen) {
			delete(g.Nodes, id)
		}
	}
	if err := SaveIndex(g, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Stats.NodeCount == 0 {
		t.Fatal("expected loaded graph")
	}
}

func TestEventIndexVersionTwo(t *testing.T) {
	root := copyWailsTestProject(t)
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(dir)
	g, meta, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if g.Stats.EventEmitCount == 0 || g.Stats.EventListenCount == 0 {
		t.Fatalf("stats emit=%d listen=%d", g.Stats.EventEmitCount, g.Stats.EventListenCount)
	}
	if err := SaveIndex(g, dir); err != nil {
		t.Fatal(err)
	}
	if err := SaveMeta(meta, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Stats.EventEmitCount == 0 {
		t.Fatal("expected event stats persisted")
	}
	_ = filepath.Join(dir, indexFileName)
}
