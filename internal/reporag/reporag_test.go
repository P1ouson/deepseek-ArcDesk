package reporag

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"arcdesk/internal/callgraph"
	"arcdesk/internal/dependency"
	"arcdesk/internal/lsp"
	"arcdesk/internal/tool"
)

func TestRepoStatusLayers(t *testing.T) {
	root := copyWailsProject(t)
	dep, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = dep.EnsureReady(context.Background())
	cg, err := callgraph.Open(root, callgraph.NewDependencyCatalog(dep))
	if err != nil {
		t.Fatal(err)
	}
	_ = cg.EnsureReady(context.Background())

	host := &Host{Root: root, Dep: dep, Callgraph: cg, CodegraphEnabled: false}
	report := host.Status(context.Background())
	if len(report.Layers) < 4 {
		t.Fatalf("layers = %+v", report.Layers)
	}
}

func TestRepoSymbolCallgraphFallback(t *testing.T) {
	root := copyWailsProject(t)
	cg, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = cg.EnsureReady(context.Background())
	host := &Host{Root: root, Callgraph: cg}
	out, err := host.SearchSymbol(context.Background(), "useSubmit", "desktop/frontend/src/lib/useSubmit.ts#useSubmit")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Symbol Graph") && !strings.Contains(out, "unavailable") {
		t.Fatalf("out=%q", out)
	}
}

func TestRepoNavigateCallgraphFallback(t *testing.T) {
	root := copyWailsProject(t)
	cg, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = cg.EnsureReady(context.Background())
	host := &Host{Root: root, Callgraph: cg}
	out, err := host.Navigate(context.Background(), "definition", "desktop/frontend/src/lib/useSubmit.ts", 1, "useSubmit")
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("expected output")
	}
}

func TestRepoToolsExecute(t *testing.T) {
	root := copyWailsProject(t)
	cg, _ := callgraph.Open(root, nil)
	_ = cg.EnsureReady(context.Background())
	reg := tool.NewRegistry()
	RegisterTools(reg, &Host{Root: root, Callgraph: cg})

	status, ok := reg.Get("repo_status")
	if !ok {
		t.Fatal("missing repo_status")
	}
	out, err := status.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "dependency") {
		t.Fatalf("out=%q err=%v", out, err)
	}

	symbol, _ := reg.Get("repo_symbol")
	out, err = symbol.Execute(context.Background(), json.RawMessage(`{"query":"Submit","file":"desktop/frontend/src/lib/useSubmit.ts"}`))
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("empty symbol output")
	}

	nav, _ := reg.Get("repo_navigate")
	_, err = nav.Execute(context.Background(), json.RawMessage(`{"file":"desktop/frontend/src/lib/useSubmit.ts","line":1,"symbol":"useSubmit"}`))
	if err != nil {
		t.Fatal(err)
	}
}

func TestRepoNavigateRequiresArgs(t *testing.T) {
	host := &Host{}
	if _, err := host.Navigate(context.Background(), "definition", "", 0, ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestRepoSymbolRequiresQuery(t *testing.T) {
	host := &Host{}
	if _, err := host.SearchSymbol(context.Background(), "", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterToolsNil(t *testing.T) {
	RegisterTools(nil, &Host{})
	RegisterTools(tool.NewRegistry(), nil)
}

func TestSplitFileSymbol(t *testing.T) {
	path, sym := splitFileSymbol("a.ts#Foo", "bar")
	if path != "a.ts" || sym != "Foo" {
		t.Fatalf("got %q %q", path, sym)
	}
}

func TestIsFrontendFile(t *testing.T) {
	if !isFrontendFile("x.tsx") || isFrontendFile("x.go") {
		t.Fatal("frontend detection")
	}
}

func TestRepoStatusWithLSP(t *testing.T) {
	host := &Host{Root: t.TempDir(), LSP: lsp.NewManager(t.TempDir(), lsp.DefaultSpecs())}
	report := host.Status(context.Background())
	found := false
	for _, layer := range report.Layers {
		if layer.Name == "code_navigation" && layer.Ready {
			found = true
		}
	}
	if !found {
		t.Fatalf("layers=%+v", report.Layers)
	}
}

func TestToolMetadata(t *testing.T) {
	st := repoStatusTool{}
	if st.Name() != "repo_status" || st.Description() == "" || !st.ReadOnly() {
		t.Fatal("status metadata")
	}
	sy := repoSymbolTool{}
	if sy.Name() != "repo_symbol" || sy.Description() == "" || !sy.ReadOnly() {
		t.Fatal("symbol metadata")
	}
	nav := repoNavigateTool{}
	if nav.Name() != "repo_navigate" || nav.Description() == "" || !nav.ReadOnly() {
		t.Fatal("navigate metadata")
	}
}

func TestCodegraphCallUnavailable(t *testing.T) {
	_, err := codegraphCall(nil, context.Background(), "codegraph_search", map[string]any{"query": "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatUnavailable(t *testing.T) {
	if !strings.Contains(formatUnavailable("symbol graph"), "unavailable") {
		t.Fatal("message")
	}
}
