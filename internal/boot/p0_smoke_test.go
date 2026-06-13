package boot

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// p0ToolNames are the agent tools required for a mature P0 coding-agent stack.
var p0ToolNames = []string{
	"repo_status", "repo_symbol", "repo_navigate",
	"dependency_status", "dependency_imports",
	"callgraph_status", "callgraph_trace_forward", "callgraph_breakpoints",
	"runtime_status", "runtime_tail",
	"selfdebug_status",
	"verification_status", "verification_plan",
	"constraint_status", "constraint_check",
	"rollback_status", "rollback_diff",
}

func TestBuildRegistersP0ToolsFromDogfoodConfig(t *testing.T) {
	home := setupBootTest(t)
	writeUserTestProvider(t, home)
	root := repoRootForP0(t)
	t.Chdir(root)

	ctrl, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	reg := ctrl.ToolRegistry()
	if reg == nil {
		t.Fatal("nil registry")
	}
	for _, name := range p0ToolNames {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("missing P0 tool %q (registry has %d tools)", name, reg.Len())
		}
	}
}

func writeUserTestProvider(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "arcdesk")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `default_model = "test-model"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func repoRootForP0(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root %q: %v", root, err)
	}
	if _, err := os.Stat(filepath.Join(root, "arcdesk.toml")); err != nil {
		t.Fatalf("missing arcdesk.toml: %v", err)
	}
	return root
}
