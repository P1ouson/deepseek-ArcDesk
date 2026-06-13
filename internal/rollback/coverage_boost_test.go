package rollback

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/checkpoint"
	"arcdesk/internal/tool"
)

func TestRollbackDiffRequiresTurn(t *testing.T) {
	host := NewHost(nil, t.TempDir(), func() int { return -1 })
	reg := tool.NewRegistry()
	RegisterTools(reg, host)
	diffTool, _ := reg.Get("rollback_diff")
	if _, err := diffTool.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error for missing turn")
	}
}

func TestFormatJSON(t *testing.T) {
	got := FormatJSON(Report{Turn: 1, Summary: "ok"})
	if got == "" || got[0] != '{' {
		t.Fatalf("got=%q", got)
	}
}

func TestBuildReportSkipsIdentical(t *testing.T) {
	root := t.TempDir()
	content := "same"
	writeFile(t, root, "a.txt", content)
	c := content
	report := BuildReport(root, checkpoint.RestorePlan{
		FromTurn: 0,
		Targets:  []checkpoint.RestoreTarget{{Path: "a.txt", Content: &c}},
	})
	if len(report.Files) != 0 {
		t.Fatalf("expected skip identical file, got %+v", report.Files)
	}
}

func TestReadWorkspaceFileEscape(t *testing.T) {
	if _, _, err := readWorkspaceFile(t.TempDir(), "../outside.txt"); err == nil {
		t.Fatal("expected escape error")
	}
}

func TestTruncateHelper(t *testing.T) {
	if got := truncate("hello world", 5); got != "he..." {
		t.Fatalf("got=%q", got)
	}
}

func TestFormatDiffBlockEmpty(t *testing.T) {
	if got := FormatDiffBlock(Report{}, 3); got != "" {
		t.Fatalf("got=%q", got)
	}
}

func TestBuildRetryContextEmpty(t *testing.T) {
	if got := BuildRetryContext(Report{}); got != "" {
		t.Fatalf("got=%q", got)
	}
}

func TestRollbackToolsMetadata(t *testing.T) {
	host := NewHost(nil, t.TempDir(), func() int { return 0 })
	reg := tool.NewRegistry()
	RegisterTools(reg, host)
	for _, name := range []string{"rollback_status", "rollback_diff"} {
		toolDef, ok := reg.Get(name)
		if !ok {
			t.Fatalf("missing %q", name)
		}
		if toolDef.Description() == "" {
			t.Fatalf("%q Description empty", name)
		}
		if !toolDef.ReadOnly() {
			t.Fatalf("%q should be read-only", name)
		}
	}
}

func TestHostActiveTurnNilCallback(t *testing.T) {
	host := NewHost(nil, t.TempDir(), nil)
	if got := host.ActiveTurn(); got != -1 {
		t.Fatalf("got=%d", got)
	}
}

func TestHostCheckpointsNilStoreResult(t *testing.T) {
	host := NewHost(func() *checkpoint.Store { return nil }, t.TempDir(), func() int { return 0 })
	if len(host.Checkpoints()) != 0 {
		t.Fatal("expected nil checkpoints")
	}
}

func TestHostReportNegativeTurn(t *testing.T) {
	host := NewHost(func() *checkpoint.Store { return checkpoint.New("", t.TempDir()) }, t.TempDir(), func() int { return 0 })
	report := host.Report(-1)
	if report.Turn != -1 || len(report.Files) != 0 {
		t.Fatalf("report=%+v", report)
	}
}

func TestRollbackDiffUsesActiveTurn(t *testing.T) {
	root := t.TempDir()
	s := checkpoint.New("", root)
	s.Begin(0, "first", 0)
	writeFile(t, root, "a.txt", "v1")
	host := NewHost(func() *checkpoint.Store { return s }, root, func() int { return 0 })
	reg := tool.NewRegistry()
	RegisterTools(reg, host)
	diffTool, _ := reg.Get("rollback_diff")
	out, err := diffTool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil || !strings.Contains(out, "Rollback diff for turn 0") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestReadWorkspaceFileAbsolutePath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "abs.txt", "content")
	abs := filepath.Join(root, "abs.txt")
	got, exists, err := readWorkspaceFile(root, abs)
	if err != nil || !exists || got != "content" {
		t.Fatalf("got=%q exists=%v err=%v", got, exists, err)
	}
}

func TestBuildFileRevertReadError(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "blocked.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "old"
	fr, ok := buildFileRevert(root, checkpoint.RestoreTarget{Path: "blocked.txt", Content: &content})
	if ok || fr.Path != "" {
		t.Fatalf("fr=%+v ok=%v", fr, ok)
	}
}

func TestTruncateNoTrimNeeded(t *testing.T) {
	if got := truncate("hi", 10); got != "hi" {
		t.Fatalf("got=%q", got)
	}
	if got := truncate("  spaced  ", 0); got != "spaced" {
		t.Fatalf("got=%q", got)
	}
}
