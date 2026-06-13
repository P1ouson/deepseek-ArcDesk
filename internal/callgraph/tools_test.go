package callgraph

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestToolsRegisterAndExecute(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)

	execute := func(name, args string) (string, error) {
		tool, ok := reg.Get(name)
		if !ok {
			t.Fatalf("tool %s not registered", name)
		}
		return tool.Execute(context.Background(), json.RawMessage(args))
	}

	status, err := execute("callgraph_status", `{}`)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(status, "nodeCount") && !strings.Contains(status, "nodes") {
		t.Fatalf("status output: %s", status)
	}

	back, err := execute("callgraph_trace_backward", `{"go_method":"Submit"}`)
	if err != nil {
		t.Fatalf("trace backward: %v", err)
	}
	if back == "" {
		t.Fatal("expected backward trace output")
	}

	fwd, err := execute("callgraph_trace_forward", `{"from":"desktop/frontend/src/components/Composer.tsx","symbol":"Composer"}`)
	if err != nil {
		t.Fatalf("trace forward: %v", err)
	}
	if fwd == "" {
		t.Fatal("expected forward trace output")
	}

	bridge, err := execute("callgraph_find_bridge", `{"frontend":"desktop/frontend/src/lib/useSubmit.ts#useSubmit","go_method":"Submit"}`)
	if err != nil {
		t.Fatalf("find bridge: %v", err)
	}
	if bridge == "" {
		t.Fatal("expected bridge path output")
	}
}

func TestFormatLLMContext(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	paths := TraceBackward(g, g.MethodMap["Submit"], DefaultTraceOptions())
	if len(paths) == 0 {
		t.Fatal("expected paths")
	}
	out := FormatLLMContext(paths, "go test ./...")
	if !strings.Contains(out, "## Wails Call Chain") {
		t.Fatalf("out = %q", out)
	}
}

func TestBuildCrossRealmContext(t *testing.T) {
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
	if block != "" && !strings.Contains(block, "## Wails Call Chain") {
		t.Fatalf("block = %q", block)
	}
}

func TestParseNodeIDAndResolve(t *testing.T) {
	root := copyWailsTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	id := NewGoBindID("desktop/app.go", "App.Submit")
	parsed, err := ParseNodeID(string(id))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path() != "desktop/app.go" {
		t.Fatalf("path = %q", parsed.Path())
	}
	resolved, err := ResolveNodeID(g, "desktop/frontend/src/components/Composer.tsx", "Composer")
	if err != nil {
		t.Fatal(err)
	}
	if resolved == "" {
		t.Fatal("expected resolved id")
	}
}

func TestIsVerifyCommand(t *testing.T) {
	if !IsVerifyCommand("go test ./...") {
		t.Fatal("expected verify")
	}
	if IsVerifyCommand("git status") {
		t.Fatal("expected non-verify")
	}
}

func TestIndexInvalidateAndMeta(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, err := Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = idx.EnsureReady(context.Background())
	if err := idx.InvalidateFiles([]string{"desktop/app.go"}); err != nil {
		t.Fatal(err)
	}
	stats, err := idx.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !stats.Stale {
		t.Fatal("expected stale after invalidate")
	}
}

func TestToolsSchemaJSON(t *testing.T) {
	root := copyWailsTestProject(t)
	idx, _ := Open(root, nil)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	for _, name := range []string{"callgraph_status", "callgraph_trace_forward", "callgraph_trace_backward", "callgraph_find_bridge", "callgraph_breakpoints"} {
		tool, ok := reg.Get(name)
		if !ok {
			t.Fatalf("missing tool %s", name)
		}
		if len(tool.Schema()) == 0 {
			t.Fatalf("empty schema for %s", name)
		}
	}
}
