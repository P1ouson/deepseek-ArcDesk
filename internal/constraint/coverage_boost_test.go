package constraint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/diff"
	"arcdesk/internal/tool"
)

func TestConstraintCheckValidation(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	reg := newTestRegistry(t)
	RegisterTools(reg, eng)
	check, _ := reg.Get("constraint_check")
	if _, err := check.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected path required error")
	}
}

func TestResultFormatEmptyBlock(t *testing.T) {
	res := Result{}
	if got := res.FormatBlockMessage(); got == "" {
		t.Fatal("expected default block message")
	}
}

func TestCheckInternalGoFrontendImport(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path:    "internal/foo/bar.go",
		Kind:    diff.Modify,
		OldText: "package foo\n",
		NewText: "package foo\n\nimport \"arcdesk/desktop/frontend/src/lib\"\n",
	})
	if !res.Blocked {
		t.Fatalf("res=%+v", res)
	}
}

func TestCheckBinarySkipped(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path:   "desktop/icon.png",
		Kind:   diff.Modify,
		Binary: true,
	})
	if res.Blocked {
		t.Fatalf("binary change should not block: %+v", res)
	}
}

func TestLastResultAndHostNil(t *testing.T) {
	var eng *Engine
	if got := eng.LastResult(); got.Blocked {
		t.Fatal("expected zero result")
	}
	eng = NewEngine(nil, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{Path: "a.go", NewText: "package main\nfunc Foo(){}"})
	if res.Blocked {
		t.Fatalf("nil host should not block: %+v", res)
	}
}

func TestListSiblingSourceFilesMissingDir(t *testing.T) {
	if files := listSiblingSourceFiles(t.TempDir(), "missing/dir", "skip.go"); len(files) != 0 {
		t.Fatalf("files=%v", files)
	}
}

func TestFindExistingUtilityModule(t *testing.T) {
	root := t.TempDir()
	if hit := findExistingUtilityModule(root); hit != "" {
		t.Fatalf("hit=%q", hit)
	}
	lib := filepathJoin(root, "desktop/frontend/src/lib")
	if err := osMkdirAll(lib); err != nil {
		t.Fatal(err)
	}
	if hit := findExistingUtilityModule(root); hit != "desktop/frontend/src/lib" {
		t.Fatalf("hit=%q", hit)
	}
}

func TestNormalizeAndPathHelpers(t *testing.T) {
	if !isFrontendPath(`desktop\frontend\src\App.tsx`) {
		t.Fatal("expected frontend path")
	}
	if !isDesktopGoPath("desktop/app.go") {
		t.Fatal("expected desktop go path")
	}
	if !isGoInternalPath("internal/agent/agent.go") {
		t.Fatal("expected internal path")
	}
}

func TestConstraintStatusEmptyViolations(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	reg := newTestRegistry(t)
	RegisterTools(reg, eng)
	status, _ := reg.Get("constraint_status")
	out, err := status.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil || !strings.Contains(out, "checks=0") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestSettingsPartialDisable(t *testing.T) {
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
		t.Fatalf("all rules disabled: %+v", res)
	}
}

func TestBuildRetryContextNoViolations(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	eng.CheckEdit("write_file", diff.Change{
		Path:    "README.md",
		Kind:    diff.Create,
		NewText: "# ok\n",
	})
	if got := BuildRetryContext(eng, []string{"README.md"}); got != "" {
		t.Fatalf("got %q", got)
	}
}

func newTestRegistry(t *testing.T) *tool.Registry {
	t.Helper()
	return tool.NewRegistry()
}

// tiny wrappers to keep coverage_boost_test import-free for os/path/filepath in one place.
func filepathJoin(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}

func osMkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

func TestConstraintToolMetadata(t *testing.T) {
	st := constraintStatusTool{}
	if st.Name() != "constraint_status" || st.Description() == "" || !st.ReadOnly() {
		t.Fatal("status metadata")
	}
	ck := constraintCheckTool{}
	if ck.Name() != "constraint_check" || ck.Description() == "" || !ck.ReadOnly() {
		t.Fatal("check metadata")
	}
}

func TestNilEngineStats(t *testing.T) {
	var eng *Engine
	if c, b, w := eng.Stats(); c != 0 || b != 0 || w != 0 {
		t.Fatalf("stats=%d %d %d", c, b, w)
	}
}

func TestLimitPathsTruncation(t *testing.T) {
	paths := []string{"a", "b", "c", "d", "e", "f", "g"}
	got := limitPaths(paths, 3)
	if len(got) != 3 || got[2] != "c" {
		t.Fatalf("got %v", got)
	}
}

func TestBuildRetryContextManyPaths(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/frontend/src/App.css",
		Kind:    diff.Modify,
		OldText: `.x{}`,
		NewText: `.x{display:none}`,
	})
	paths := make([]string, 10)
	for i := range paths {
		paths[i] = fmt.Sprintf("p%d.css", i)
	}
	got := BuildRetryContext(eng, paths)
	if !strings.Contains(got, "p0.css") || strings.Contains(got, "p9.css") {
		t.Fatalf("got %q", got)
	}
}

func TestCheckClassNameTweakBlock(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/frontend/src/components/Panel.tsx",
		Kind:    diff.Modify,
		OldText: `export function Panel() { return <div className="a"/> }`,
		NewText: `export function Panel() { return <div className="b"/> }`,
	})
	if !res.Blocked {
		t.Fatalf("res=%+v", res)
	}
}

func TestCheckDuplicateSiblingFile(t *testing.T) {
	root := t.TempDir()
	dir := filepathJoin(root, "desktop/frontend/src/lib")
	if err := osMkdirAll(dir); err != nil {
		t.Fatal(err)
	}
	existing := filepathJoin(dir, "helpers.ts")
	if err := os.WriteFile(existing, []byte("export function formatHelper(x: string) { return x }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(&Host{Root: root}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/frontend/src/lib/extra.ts",
		Kind:    diff.Create,
		NewText: "export function formatHelper(x: string) { return x.trim() }\n",
	})
	if !res.Blocked {
		t.Fatalf("res=%+v", res)
	}
}

func TestFindSymbolInFilesReadError(t *testing.T) {
	if hit := findSymbolInFiles(t.TempDir(), []string{"missing/file.go"}, "Foo"); hit != "" {
		t.Fatalf("hit=%q", hit)
	}
}

func TestCheckReuseNewUtilFile(t *testing.T) {
	root := t.TempDir()
	lib := filepathJoin(root, "desktop/frontend/src/lib")
	if err := osMkdirAll(lib); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(&Host{Root: root}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/frontend/src/utils.ts",
		Kind:    diff.Create,
		NewText: "export function noop() {}\n",
	})
	if len(res.Violations) == 0 {
		t.Fatalf("res=%+v", res)
	}
}

func TestCheckArchitectureEmptyAdded(t *testing.T) {
	if v := checkArchitecture("internal/foo.go", ""); len(v) != 0 {
		t.Fatalf("v=%v", v)
	}
}

func TestConstraintCheckInvalidJSON(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	reg := newTestRegistry(t)
	RegisterTools(reg, eng)
	check, _ := reg.Get("constraint_check")
	if _, err := check.Execute(context.Background(), json.RawMessage(`{`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestConstraintCheckBlockedWithHint(t *testing.T) {
	eng := NewEngine(&Host{Root: t.TempDir()}, DefaultSettings())
	reg := newTestRegistry(t)
	RegisterTools(reg, eng)
	check, _ := reg.Get("constraint_check")
	out, err := check.Execute(context.Background(), json.RawMessage(`{
	  "path":"desktop/frontend/src/App.css",
	  "old_text":".x{}",
	  "new_text":".x{display:none}"
	}`))
	if err != nil || !strings.Contains(out, "blocked=true") || !strings.Contains(out, "CSS-only") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestCheckPathNilEngine(t *testing.T) {
	var eng *Engine
	if res := eng.CheckPath("a.go", "", "package main\n"); res.Blocked || res.Path != "" {
		t.Fatalf("res=%+v", res)
	}
}

func TestAddedLinesNoChange(t *testing.T) {
	if got := addedLines("same\nline", "same\nline"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestCheckDuplicateEmptyRoot(t *testing.T) {
	eng := NewEngine(&Host{Root: ""}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path:    "a.go",
		Kind:    diff.Create,
		NewText: "package main\nfunc Dup() {}\n",
	})
	if res.Blocked {
		t.Fatalf("res=%+v", res)
	}
}

func TestCheckReuseEmptyRoot(t *testing.T) {
	eng := NewEngine(&Host{Root: ""}, DefaultSettings())
	res := eng.CheckEdit("write_file", diff.Change{
		Path:    "desktop/frontend/src/utils.ts",
		Kind:    diff.Create,
		NewText: "export function formatHelper() {}\n",
	})
	if len(res.Violations) > 0 {
		t.Fatalf("res=%+v", res)
	}
}
