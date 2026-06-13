package dependency

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func readyTestIndex(t *testing.T) *Index {
	t.Helper()
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestRegisterTools(t *testing.T) {
	idx := readyTestIndex(t)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)

	names := []string{"dependency_status", "dependency_affected_by", "dependency_imports", "dependency_cycles"}
	for _, name := range names {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("missing tool %q", name)
		}
	}
}

func TestDependencyStatusTool(t *testing.T) {
	idx := readyTestIndex(t)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	tool, _ := reg.Get("dependency_status")

	out, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Dependency index:") {
		t.Fatalf("unexpected output: %q", out)
	}
	if !strings.Contains(out, `"nodeCount"`) {
		t.Fatalf("expected JSON stats: %q", out)
	}
	if !strings.Contains(out, "orphans=") {
		t.Fatalf("expected orphans in summary: %q", out)
	}
}

func TestDependencyAffectedByTool(t *testing.T) {
	idx := readyTestIndex(t)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	tool, _ := reg.Get("dependency_affected_by")

	out, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"internal/alpha/alpha.go"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Impact for") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestDependencyImportsTool(t *testing.T) {
	idx := readyTestIndex(t)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	tool, _ := reg.Get("dependency_imports")

	out, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"internal/alpha","direction":"imports"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "imports") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestDependencyCyclesTool(t *testing.T) {
	idx := readyTestIndex(t)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	tool, _ := reg.Get("dependency_cycles")

	out, err := tool.Execute(context.Background(), json.RawMessage(`{"lang":"all"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Dependency cycles:") {
		t.Fatalf("unexpected output: %q", out)
	}
}
