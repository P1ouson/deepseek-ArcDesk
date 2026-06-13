package rollback

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/checkpoint"
	"arcdesk/internal/diff"
	"arcdesk/internal/tool"
)

func TestBuildReportModifiedFile(t *testing.T) {
	root := t.TempDir()
	path := "a.txt"
	writeFile(t, root, path, "v2")

	plan := checkpoint.RestorePlan{
		FromTurn: 0,
		Prompt:   "fix bug",
		Targets: []checkpoint.RestoreTarget{{
			Path:    path,
			Content: strPtr("v0"),
			Turn:    0,
		}},
	}
	report := BuildReport(root, plan)
	if len(report.Files) != 1 || report.Files[0].Action != "modify" {
		t.Fatalf("report=%+v", report)
	}
	if report.Files[0].Removed == 0 {
		t.Fatalf("expected removed lines in diff: %+v", report.Files[0])
	}
}

func TestBuildReportCreatedFile(t *testing.T) {
	root := t.TempDir()
	path := "new.txt"
	writeFile(t, root, path, "hello")

	plan := checkpoint.RestorePlan{
		FromTurn: 1,
		Targets: []checkpoint.RestoreTarget{{
			Path: path,
			Turn: 1,
		}},
	}
	report := BuildReport(root, plan)
	if len(report.Files) != 1 || report.Files[0].Action != "create" {
		t.Fatalf("report=%+v", report)
	}
}

func TestBuildReportDeletedFile(t *testing.T) {
	root := t.TempDir()
	path := "gone.txt"
	content := "restore-me"
	plan := checkpoint.RestorePlan{
		FromTurn: 0,
		Targets: []checkpoint.RestoreTarget{{
			Path:    path,
			Content: strPtr(content),
			Turn:    0,
		}},
	}
	report := BuildReport(root, plan)
	if len(report.Files) != 1 || report.Files[0].Action != "delete" {
		t.Fatalf("report=%+v", report)
	}
}

func TestHostReportWithoutStore(t *testing.T) {
	host := NewHost(func() *checkpoint.Store { return nil }, t.TempDir(), func() int { return 0 })
	report := host.Report(0)
	if len(report.Files) != 0 {
		t.Fatalf("report=%+v", report)
	}
}

func TestReadWorkspaceFileEmptyPath(t *testing.T) {
	if _, _, err := readWorkspaceFile(t.TempDir(), "  "); err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatAutoNoticeEmptyReport(t *testing.T) {
	got := FormatAutoNotice(Report{Turn: 1})
	if !strings.Contains(got, "turn 1") {
		t.Fatalf("got=%q", got)
	}
}

func TestFormatAutoNoticeAndRetryContext(t *testing.T) {
	report := Report{
		Turn:    2,
		Summary: "1 file(s): 1 modified, 0 created, 0 deleted",
		Files: []FileRevert{{
			Path: "a.go", Action: "modify", Removed: 2, Added: 1,
			Diff: "--- a.go\n+++ a.go\n@@ -1 +1 @@\n-old\n+new",
		}},
	}
	notice := FormatAutoNotice(report)
	if !strings.Contains(notice, "rewound turn 2") || !strings.Contains(notice, "Reverted changes") {
		t.Fatalf("notice=%q", notice)
	}
	retry := BuildRetryContext(report)
	if !strings.Contains(retry, "## Rollback Preview") {
		t.Fatalf("retry=%q", retry)
	}
}

func TestRollbackTools(t *testing.T) {
	root := t.TempDir()
	s := checkpoint.New("", root)
	s.Begin(0, "first", 0)
	s.Snapshot(diff.Change{Path: "a.txt", Kind: diff.Modify, OldText: "v0"})
	writeFile(t, root, "a.txt", "v1")

	host := NewHost(func() *checkpoint.Store { return s }, root, func() int { return 0 })
	reg := tool.NewRegistry()
	RegisterTools(reg, host)

	status, ok := reg.Get("rollback_status")
	if !ok {
		t.Fatal("missing rollback_status")
	}
	out, err := status.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil || !strings.Contains(out, "turn 0") {
		t.Fatalf("out=%q err=%v", out, err)
	}

	diffTool, ok := reg.Get("rollback_diff")
	if !ok {
		t.Fatal("missing rollback_diff")
	}
	out, err = diffTool.Execute(context.Background(), json.RawMessage(`{"turn":0}`))
	if err != nil || !strings.Contains(out, "Rollback diff") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestHostNilSafe(t *testing.T) {
	var h *Host
	if len(h.Checkpoints()) != 0 {
		t.Fatal("expected nil checkpoints")
	}
	if got := h.Report(0); got.Turn != 0 || len(got.Files) != 0 {
		t.Fatalf("got=%+v", got)
	}
}

func TestFormatDiffBlockTruncation(t *testing.T) {
	report := Report{Files: []FileRevert{{Path: "a"}, {Path: "b"}, {Path: "c"}}}
	block := FormatDiffBlock(report, 1)
	if !strings.Contains(block, "2 more file") {
		t.Fatalf("block=%q", block)
	}
}

func TestRegisterToolsNil(t *testing.T) {
	RegisterTools(nil, NewHost(nil, "", nil))
	RegisterTools(tool.NewRegistry(), nil)
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func strPtr(s string) *string { return &s }
