package guardian

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"strings"

	"arcdesk/internal/archrag"
	"arcdesk/internal/constraint"
	"arcdesk/internal/tool"
)

func TestCompileRulesFromSPEC(t *testing.T) {
	dir := t.TempDir()
	spec := "## Rules\n\n" +
		"- Prefer editing `internal/counter` instead of duplicating logic in `desktop/`.\n" +
		"- Wails bind methods live under `desktop/` only.\n" +
		"- Do not fix UI with hardcoded mock data.\n"
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := archrag.NewIndex(dir, nil)
	rules := CompileRules(idx)
	if len(rules) < 3 {
		t.Fatalf("rules = %d, want >= 3", len(rules))
	}
}

func TestGuardianBlocksWailsBindOutsideDesktop(t *testing.T) {
	dir := t.TempDir()
	spec := "## Rules\n\n- Wails bind methods live under `desktop/` only.\n"
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := archrag.NewIndex(dir, nil)
	host := &constraint.Host{Root: dir}
	eng := constraint.NewEngine(host, constraint.DefaultSettings())
	g := New(idx, eng)

	res := g.CheckPath("internal/counter/app.go", "", "func (a *App) Submit() {}\n")
	if !res.Blocked {
		t.Fatalf("expected block, got %+v", res)
	}
}

func TestGuardianToolsRegister(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte("## Rules\n\n- Prefer `internal/`.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := archrag.NewIndex(dir, nil)
	g := New(idx, nil)
	reg := tool.NewRegistry()
	RegisterTools(reg, g)
	for _, name := range []string{"architecture_guardian_status", "architecture_guardian_rules", "architecture_guardian_check"} {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("missing tool %q", name)
		}
	}
	status, _ := reg.Get("architecture_guardian_status")
	out, err := status.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil || out == "" {
		t.Fatalf("status: %q err=%v", out, err)
	}
}

func TestBuildRetryContextIncludesRules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte("## Rules\n\n- Keep logic in `internal/`.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := New(archrag.NewIndex(dir, nil), nil)
	got := BuildRetryContext(g, []string{"internal/counter/counter.go"})
	if got == "" || !strings.Contains(got, "Architecture Guardian") {
		t.Fatalf("context = %q", got)
	}
}
