package constraint

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/diff"
	"arcdesk/internal/tool"
)

func TestCheckDuplicateSymbol(t *testing.T) {
	root := copyWailsProject(t)
	eng := NewEngine(&Host{Root: root}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/app_extra.go",
		Kind:    diff.Create,
		NewText: "package main\n\nfunc (a *App) Submit(msg string) error { return nil }\n",
	})
	if !res.Blocked {
		t.Fatalf("expected duplicate Submit block, got %+v", res)
	}
}

func TestCheckFakeUIBlock(t *testing.T) {
	root := t.TempDir()
	eng := NewEngine(&Host{Root: root}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path: "desktop/frontend/src/components/Panel.tsx",
		Kind: diff.Modify,
		OldText: `export function Panel() { return <div/> }`,
		NewText: `export function Panel() {
  const mockData = { ok: true };
  return <div>{mockData.ok ? "ok" : "fail"}</div>
}`,
	})
	if !res.Blocked {
		t.Fatalf("expected fake UI block, got %+v", res)
	}
}

func TestCheckCSSOnlyBlock(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/frontend/src/App.css",
		Kind:    diff.Modify,
		OldText: `.err { color: red; }`,
		NewText: `.err { display: none; }`,
	})
	if !res.Blocked {
		t.Fatalf("expected css-only block, got %+v", res)
	}
}

func TestCheckArchitectureComponentWailsImport(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/frontend/src/components/Bad.tsx",
		Kind:    diff.Modify,
		OldText: `export function Bad() { return null }`,
		NewText: `import { Submit } from "../../wailsjs/go/main/App";
export function Bad() { return null }`,
	})
	if !res.Blocked {
		t.Fatalf("expected architecture block, got %+v", res)
	}
}

func TestCheckArchitectureBindOutsideDesktop(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path:    "internal/agent/helper.go",
		Kind:    diff.Modify,
		OldText: "package agent\n",
		NewText: "package agent\n\nfunc (a *App) DoThing() error { return nil }\n",
	})
	if !res.Blocked {
		t.Fatalf("expected bind placement block, got %+v", res)
	}
}

func TestCheckReuseWarn(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "desktop", "frontend", "src", "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(&Host{Root: root}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/frontend/src/utils.ts",
		Kind:    diff.Create,
		NewText: "export function formatHelper(x: string) { return x.trim() }\n",
	})
	if res.Blocked {
		t.Fatalf("reuse should warn not block: %+v", res)
	}
	if hint := res.FormatWarnHint(); !strings.Contains(hint, "[constraint]") {
		t.Fatalf("expected warn hint, got %q", hint)
	}
}

func TestBridgeWiringAllowsFunctionalFix(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path: "desktop/frontend/src/lib/useSubmit.ts",
		Kind: diff.Modify,
		OldText: `export function useSubmit() { return async () => {} }`,
		NewText: `import { app } from "./bridge";
export function useSubmit() {
  return async () => { await app.Submit("hi"); };
}`,
	})
	for _, v := range res.Violations {
		if v.Rule == RuleFakeUI && v.Severity == SeverityBlock {
			t.Fatalf("bridge wiring should not trigger fake UI block: %+v", res)
		}
	}
}

func TestConstraintTools(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/frontend/src/App.css",
		Kind:    diff.Modify,
		OldText: `.x{}`,
		NewText: `.x{display:none}`,
	})
	reg := tool.NewRegistry()
	RegisterTools(reg, eng)

	status, ok := reg.Get("constraint_status")
	if !ok {
		t.Fatal("missing constraint_status")
	}
	out, err := status.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil || !strings.Contains(out, "Constraint system") {
		t.Fatalf("status out=%q err=%v", out, err)
	}

	check, ok := reg.Get("constraint_check")
	if !ok {
		t.Fatal("missing constraint_check")
	}
	out, err = check.Execute(context.Background(), json.RawMessage(`{
	  "path":"desktop/frontend/src/components/X.tsx",
	  "old_text":"export function X(){return null}",
	  "new_text":"export function X(){ const fakeSuccess = true; return fakeSuccess ? 1 : 0 }"
	}`))
	if err != nil || !strings.Contains(out, "blocked=true") {
		t.Fatalf("check out=%q err=%v", out, err)
	}
}

func TestBuildRetryContext(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/frontend/src/App.css",
		Kind:    diff.Modify,
		OldText: `.a{}`,
		NewText: `.a{display:none}`,
	})
	got := BuildRetryContext(eng, []string{"desktop/frontend/src/App.css"})
	if !strings.Contains(got, "## Constraint System") {
		t.Fatalf("got %q", got)
	}
}

func TestNilEngineSafe(t *testing.T) {
	var eng *Engine
	if got := BuildRetryContext(eng, []string{"a"}); got != "" {
		t.Fatalf("got %q", got)
	}
	res := eng.CheckEdit("write_file", diff.Change{Path: "a.go", NewText: "package main"})
	if res.Blocked || len(res.Violations) > 0 {
		t.Fatalf("nil engine should pass: %+v", res)
	}
}

func TestFormatBlockMessage(t *testing.T) {
	res := Result{
		Blocked: true,
		Violations: []Violation{{
			Severity: SeverityBlock,
			Message:  "bad",
			Hint:     "fix it",
		}},
	}
	got := res.FormatBlockMessage()
	if !strings.Contains(got, "bad") || !strings.Contains(got, "fix it") {
		t.Fatalf("got %q", got)
	}
}

func TestAddedLinesAndSymbols(t *testing.T) {
	added := addedLines("a\nb", "a\nc\nfunc NewThing() {}\n")
	if !strings.Contains(added, "func NewThing") {
		t.Fatalf("added=%q", added)
	}
	syms := extractSymbols("x.go", added)
	if len(syms) != 1 || syms[0] != "NewThing" {
		t.Fatalf("syms=%v", syms)
	}
}

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	if !s.BlockDuplicate || !s.BlockFakeUI || !s.BlockArch || !s.AdviseReuse {
		t.Fatalf("defaults=%+v", s)
	}
}

func TestEngineStats(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/frontend/src/App.css",
		Kind:    diff.Modify,
		OldText: `.x{}`,
		NewText: `.x{display:none}`,
	})
	checks, blocks, warnings := eng.Stats()
	if checks != 1 || blocks != 1 {
		t.Fatalf("checks=%d blocks=%d warnings=%d", checks, blocks, warnings)
	}
}

func TestRegisterToolsNil(t *testing.T) {
	RegisterTools(nil, NewEngine(&Host{Root: t.TempDir()}, DefaultSettings()))
	RegisterTools(tool.NewRegistry(), nil)
}

func TestCheckPathCreateKind(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	res := eng.CheckPath("desktop/frontend/src/App.css", "", ".x{display:none}")
	if !res.Blocked {
		t.Fatalf("res=%+v", res)
	}
}

func TestDisabledRules(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, Settings{
		BlockDuplicate: false,
		BlockFakeUI:    false,
		BlockArch:      false,
		AdviseReuse:    false,
	})
	res := eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/frontend/src/App.css",
		Kind:    diff.Modify,
		OldText: `.x{}`,
		NewText: `.x{display:none}`,
	})
	if res.Blocked || len(res.Violations) > 0 {
		t.Fatalf("disabled rules should pass: %+v", res)
	}
}
