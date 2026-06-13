package reporag

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/callgraph"
	"arcdesk/internal/dependency"
	"arcdesk/internal/lsp"
	"arcdesk/internal/repomap"
	"arcdesk/internal/tool"
)

type stubTool struct {
	name     string
	desc     string
	readOnly bool
	out      string
	err      error
}

func (s stubTool) Name() string            { return s.name }
func (s stubTool) Description() string     { return s.desc }
func (s stubTool) ReadOnly() bool          { return s.readOnly }
func (s stubTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (s stubTool) Execute(context.Context, json.RawMessage) (string, error) {
	return s.out, s.err
}

func TestRegistryCallSuccess(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(stubTool{name: "mcp__codegraph__codegraph_search", out: `{"results":[{"name":"Foo"}]}`})
	out, used, err := registryCall(reg, context.Background(), []string{"mcp__codegraph__codegraph_search"}, json.RawMessage(`{}`))
	if err != nil || used == "" || out == "" {
		t.Fatalf("out=%q used=%q err=%v", out, used, err)
	}
}

func TestSearchSymbolCodegraphPath(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(stubTool{name: "mcp__codegraph__codegraph_search", out: "symbol hits"})
	host := &Host{Root: t.TempDir(), Reg: reg, CodegraphEnabled: true}
	out, err := host.SearchSymbol(context.Background(), "Submit", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CodeGraph") {
		t.Fatalf("out=%q", out)
	}
}

func TestNavigateCodegraphCallers(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(stubTool{name: "mcp__codegraph__codegraph_callers", out: "callers list"})
	host := &Host{Root: t.TempDir(), Reg: reg, CodegraphEnabled: true}
	out, err := host.Navigate(context.Background(), "references", "main.go", 10, "Foo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CodeGraph") {
		t.Fatalf("out=%q", out)
	}
}

func TestNavigateCodegraphSearchFallback(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(stubTool{name: "mcp__codegraph__codegraph_node", err: errors.New("miss")})
	reg.Add(stubTool{name: "mcp__codegraph__codegraph_search", out: "search hit"})
	host := &Host{Root: t.TempDir(), Reg: reg, CodegraphEnabled: true}
	out, err := host.Navigate(context.Background(), "definition", "main.go", 10, "Foo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "search") {
		t.Fatalf("out=%q", out)
	}
}

func TestNavigateUnavailableFallback(t *testing.T) {
	root := t.TempDir()
	host := &Host{Root: root}
	out, err := host.Navigate(context.Background(), "definition", "main.go", 1, "Foo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "unavailable") {
		t.Fatalf("out=%q", out)
	}
}

func TestStatusCodegraphInitialized(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".codegraph"), 0o755); err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root, CodegraphEnabled: true}
	report := host.Status(context.Background())
	for _, layer := range report.Layers {
		if layer.Name == "symbol_graph" && !layer.Ready {
			t.Fatalf("layers=%+v", report.Layers)
		}
	}
}

func TestStatusCodegraphMCPConnected(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(stubTool{name: "mcp__codegraph__codegraph_search", out: "ok"})
	host := &Host{Root: t.TempDir(), Reg: reg, CodegraphEnabled: true}
	for _, layer := range host.Status(context.Background()).Layers {
		if layer.Name == "symbol_graph" && (!layer.Ready || !strings.Contains(layer.Detail, "connected")) {
			t.Fatalf("layer=%+v", layer)
		}
	}
}

func TestStatusCallgraphDisabled(t *testing.T) {
	host := &Host{Root: t.TempDir()}
	for _, layer := range host.Status(context.Background()).Layers {
		if layer.Name == "callgraph" && layer.Detail != "disabled" {
			t.Fatalf("detail=%q", layer.Detail)
		}
	}
}

func TestSearchSymbolEmptyCodegraphFallsThrough(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(stubTool{name: "mcp__codegraph__codegraph_search", out: "   "})
	root := copyWailsProject(t)
	cg, _ := callgraph.Open(root, nil)
	_ = cg.EnsureReady(context.Background())
	host := &Host{Root: root, Reg: reg, CodegraphEnabled: true, Callgraph: cg}
	out, err := host.SearchSymbol(context.Background(), "useSubmit", "desktop/frontend/src/lib/useSubmit.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(out), "callgraph") {
		t.Fatalf("out=%q", out)
	}
}

func TestRegistryCallNilReg(t *testing.T) {
	_, _, err := registryCall(nil, context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRegistryCallExecuteError(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(stubTool{name: "mcp__codegraph__codegraph_search", err: errors.New("boom")})
	_, _, err := registryCall(reg, context.Background(), []string{"mcp__codegraph__codegraph_search"}, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNavigateFrontendNonTs(t *testing.T) {
	host := &Host{Root: t.TempDir()}
	out, err := host.Navigate(context.Background(), "definition", "main.go", 1, "Foo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "unavailable") {
		t.Fatalf("out=%q", out)
	}
}

func TestStatusNilHost(t *testing.T) {
	var h *Host
	if len(h.Status(context.Background()).Layers) != 0 {
		t.Fatal("expected empty")
	}
}

func TestStatusRepomapReady(t *testing.T) {
	root := t.TempDir()
	if err := repomap.EnsureReady(root); err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root}
	report := host.Status(context.Background())
	for _, layer := range report.Layers {
		if layer.Name == "repomap" && !layer.Ready {
			t.Fatalf("layers=%+v", report.Layers)
		}
	}
}

func TestRepoSymbolToolEmptyQuery(t *testing.T) {
	tool := repoSymbolTool{host: &Host{Root: t.TempDir()}}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"query":""}`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestStatusDepEnsurePath(t *testing.T) {
	root := copyWailsProject(t)
	dep, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root, Dep: dep}
	report := host.Status(context.Background())
	if len(report.Layers) == 0 {
		t.Fatal("expected layers")
	}
}

func TestSearchSymbolDependencyModule(t *testing.T) {
	root := copyWailsProject(t)
	dep, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = dep.EnsureReady(context.Background())
	host := &Host{Root: root, Dep: dep}
	out, err := host.SearchSymbol(context.Background(), "desktop", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Dependency module") && !strings.Contains(out, "unavailable") {
		t.Fatalf("out=%q", out)
	}
}

func TestSearchSymbolCallgraphSuccess(t *testing.T) {
	root := copyWailsProject(t)
	cg, _ := callgraph.Open(root, nil)
	_ = cg.EnsureReady(context.Background())
	host := &Host{Root: root, Callgraph: cg}
	out, err := host.SearchSymbol(context.Background(), "useSubmit", "desktop/frontend/src/lib/useSubmit.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "callgraph") {
		t.Fatalf("out=%q", out)
	}
}

func TestRepoNavigateToolInvalidJSON(t *testing.T) {
	tool := repoNavigateTool{host: &Host{}}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestRepoStatusToolExecute(t *testing.T) {
	host := &Host{Root: t.TempDir(), LSP: lsp.NewManager(t.TempDir(), lsp.DefaultSpecs())}
	tool := repoStatusTool{host: host}
	out, err := tool.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "code_navigation") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestHostSearchSymbolNil(t *testing.T) {
	var h *Host
	if _, err := h.SearchSymbol(context.Background(), "x", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestCodegraphCallMarshalError(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(stubTool{name: "codegraph_search", out: "ok"})
	if _, err := codegraphCall(reg, context.Background(), "codegraph_search", map[string]any{"x": make(chan int)}); err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestSplitFileSymbolFallback(t *testing.T) {
	path, sym := splitFileSymbol("", "query")
	if path != "" || sym != "query" {
		t.Fatalf("got %q %q", path, sym)
	}
}

func TestRegistryCallFallbackToolName(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(stubTool{name: "codegraph_search", out: "symbol hits"})
	out, used, err := registryCall(reg, context.Background(), []string{
		"mcp__codegraph__codegraph_search",
		"codegraph_search",
	}, json.RawMessage(`{}`))
	if err != nil || used != "codegraph_search" || out == "" {
		t.Fatalf("out=%q used=%q err=%v", out, used, err)
	}
}

func TestRegistryCallNoMatchingTool(t *testing.T) {
	reg := tool.NewRegistry()
	_, _, err := registryCall(reg, context.Background(), []string{"missing_tool"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNavigateNilHost(t *testing.T) {
	var h *Host
	if _, err := h.Navigate(context.Background(), "definition", "main.ts", 1, "Foo"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNavigateDefaultModeAndRefsAliases(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(stubTool{name: "mcp__codegraph__codegraph_node", out: "definition hit"})
	reg.Add(stubTool{name: "mcp__codegraph__codegraph_callers", out: "reference hit"})
	host := &Host{Root: t.TempDir(), Reg: reg, CodegraphEnabled: true}

	out, err := host.Navigate(context.Background(), "", "main.ts", 1, "Foo")
	if err != nil || !strings.Contains(out, "CodeGraph") {
		t.Fatalf("default mode out=%q err=%v", out, err)
	}
	for _, mode := range []string{"refs", "reference"} {
		out, err = host.Navigate(context.Background(), mode, "main.ts", 1, "Foo")
		if err != nil || !strings.Contains(out, "CodeGraph") {
			t.Fatalf("mode=%s out=%q err=%v", mode, out, err)
		}
	}
}

func TestNavigateCallgraphSuccess(t *testing.T) {
	root := copyWailsProject(t)
	cg, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := cg.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root, Callgraph: cg}
	out, err := host.Navigate(context.Background(), "definition", "desktop/frontend/src/lib/useSubmit.ts", 1, "useSubmit")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Wails callgraph") {
		t.Fatalf("out=%q", out)
	}
}

func TestSearchSymbolDependencyResolved(t *testing.T) {
	root := copyWailsProject(t)
	dep, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := dep.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root, Dep: dep}
	out, err := host.SearchSymbol(context.Background(), "desktop", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Dependency module") && !strings.Contains(out, "unavailable") {
		t.Fatalf("out=%q", out)
	}
}

func TestSearchSymbolSplitFileAnchor(t *testing.T) {
	root := copyWailsProject(t)
	cg, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := cg.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root, Callgraph: cg}
	out, err := host.SearchSymbol(context.Background(), "ignored", "desktop/frontend/src/lib/useSubmit.ts#useSubmit")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(out), "callgraph") && !strings.Contains(out, "unavailable") {
		t.Fatalf("out=%q", out)
	}
}

func TestRepoStatusStaleCallgraph(t *testing.T) {
	root := copyWailsProject(t)
	cg, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := cg.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root, Callgraph: cg}
	report := host.Status(context.Background())
	for _, layer := range report.Layers {
		if layer.Name == "callgraph" && layer.Detail == "disabled" {
			t.Fatalf("layers=%+v", report.Layers)
		}
	}
}

func TestRepoStatusToolStaleLayer(t *testing.T) {
	root := copyWailsProject(t)
	cg, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := cg.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root, Callgraph: cg}
	tool := repoStatusTool{host: host}
	out, err := tool.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "callgraph") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestRepoSymbolToolEmptyArgs(t *testing.T) {
	tool := repoSymbolTool{host: &Host{Root: t.TempDir()}}
	if _, err := tool.Execute(context.Background(), nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestIsFrontendFileExtensions(t *testing.T) {
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx"} {
		if !isFrontendFile("component" + ext) {
			t.Fatalf("expected frontend for %s", ext)
		}
	}
	if isFrontendFile("main.go") {
		t.Fatal("go file should not be frontend")
	}
}

func TestStatusCodegraphNotInitialized(t *testing.T) {
	host := &Host{Root: t.TempDir(), CodegraphEnabled: true}
	for _, layer := range host.Status(context.Background()).Layers {
		if layer.Name == "symbol_graph" && layer.Ready {
			t.Fatalf("expected not ready without init: %+v", layer)
		}
		if layer.Name == "symbol_graph" && !strings.Contains(layer.Detail, "codegraph init") {
			t.Fatalf("detail=%q", layer.Detail)
		}
	}
}

func TestSearchSymbolDependencyModuleResolved(t *testing.T) {
	root := copyGoTestProject(t)
	dep, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := dep.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root, Dep: dep}
	out, err := host.SearchSymbol(context.Background(), "internal/gamma/gamma.go", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Dependency module") {
		t.Fatalf("out=%q", out)
	}
}

func TestStatusDepEnsureReadyFallback(t *testing.T) {
	root := copyGoTestProject(t)
	dep, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root, Dep: dep}
	report := host.Status(context.Background())
	for _, layer := range report.Layers {
		if layer.Name == "dependency" && (!layer.Ready || layer.Detail != "index ready") {
			t.Fatalf("layer=%+v", layer)
		}
	}
}

func TestRepoStatusToolStaleCallgraph(t *testing.T) {
	root := copyWailsProject(t)
	cg, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := cg.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := cg.InvalidateFiles([]string{"desktop/app.go"}); err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root, Callgraph: cg}
	out, err := repoStatusTool{host: host}.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "stale") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestCodegraphCallExecuteError(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(stubTool{name: "mcp__codegraph__codegraph_search", err: errors.New("boom")})
	if _, err := codegraphCall(reg, context.Background(), "codegraph_search", map[string]any{"query": "x"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNavigateLSPBranches(t *testing.T) {
	root := copyWailsProject(t)
	mgr := lsp.NewManager(root, lsp.DefaultSpecs())
	t.Cleanup(mgr.Close)
	host := &Host{Root: root, LSP: mgr}
	for _, mode := range []string{"definition", "references"} {
		out, err := host.Navigate(context.Background(), mode, "desktop/app.go", 5, "Submit")
		if err != nil {
			t.Fatalf("mode=%s err=%v", mode, err)
		}
		if out == "" {
			t.Fatalf("mode=%s empty output", mode)
		}
	}
}

func TestNavigateSkipsCallgraphForGoFile(t *testing.T) {
	root := copyWailsProject(t)
	cg, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := cg.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root, Callgraph: cg}
	out, err := host.Navigate(context.Background(), "definition", "desktop/app.go", 1, "Submit")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Wails callgraph") {
		t.Fatalf("go file should not use callgraph: %q", out)
	}
}

func TestCodegraphCallUsedEmpty(t *testing.T) {
	reg := tool.NewRegistry()
	if _, err := codegraphCall(reg, context.Background(), "codegraph_search", map[string]any{"query": "x"}); err == nil {
		t.Fatal("expected unavailable error")
	}
}
