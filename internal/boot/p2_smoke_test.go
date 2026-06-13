package boot

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// p2ToolNames are the agent tools required for the P2 differentiation stack.
var p2ToolNames = []string{
	"runtime_find",
	"ui_status", "ui_list", "ui_find", "ui_read",
	"architecture_guardian_status", "architecture_guardian_rules", "architecture_guardian_check",
	"taskdag_status", "taskdag_load", "taskdag_ready", "taskdag_start", "taskdag_complete",
	"cost_router_status", "cost_router_classify",
	"context_compression_status",
}

func TestBuildRegistersP2ToolsFromRepoRoot(t *testing.T) {
	home := setupBootTest(t)
	writeUserTestProvider(t, home)
	root := repoRootForP2(t)
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
	for _, name := range p2ToolNames {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("missing P2 tool %q (registry has %d tools)", name, reg.Len())
		}
	}
}

func repoRootForP2(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root %q: %v", root, err)
	}
	if _, err := os.Stat(filepath.Join(root, "desktop", "frontend", "src")); err != nil {
		t.Fatalf("expected desktop frontend for ui_rag smoke: %v", err)
	}
	return root
}
