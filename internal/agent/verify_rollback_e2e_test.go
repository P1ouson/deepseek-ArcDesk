package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/checkpoint"
	"arcdesk/internal/diff"
	"arcdesk/internal/event"
	"arcdesk/internal/instruction"
	"arcdesk/internal/provider"
	"arcdesk/internal/rollback"
	"arcdesk/internal/tool"
)

type diskWriteTool struct {
	root string
}

func (d diskWriteTool) Name() string            { return "write_file" }
func (d diskWriteTool) Description() string     { return "" }
func (d diskWriteTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (d diskWriteTool) ReadOnly() bool          { return false }

func (d diskWriteTool) Preview(args json.RawMessage) (diff.Change, error) {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return diff.Change{}, err
	}
	oldText, _ := os.ReadFile(filepath.Join(d.root, filepath.FromSlash(in.Path)))
	return diff.Change{
		Path: in.Path, Kind: diff.Modify,
		OldText: string(oldText), NewText: in.Content,
	}, nil
}

func (d diskWriteTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", err
	}
	abs := filepath.Join(d.root, filepath.FromSlash(in.Path))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, []byte(in.Content), 0o644); err != nil {
		return "", err
	}
	return "written", nil
}

func TestVerifyRollbackEndToEndWithDiff(t *testing.T) {
	root := t.TempDir()
	rel := "alpha.go"
	good := "package alpha\n\nconst Version = \"good\"\n"
	if err := os.WriteFile(filepath.Join(root, rel), []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}

	cp := checkpoint.New("", root)
	cp.Begin(0, "fix alpha", 0)

	reg := tool.NewRegistry()
	reg.Add(diskWriteTool{root: root})

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("w1", "write_file", `{"path":"alpha.go","content":"package alpha\n\nconst Version = \"broken\"\n"}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done without verify 1"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "done without verify 2"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{
		ProjectChecks:    []instruction.VerifyCheck{{Command: "go test ./...", SourcePath: "arcdesk.toml"}},
		VerifyMaxRetries: 2,
		VerifyOnFailure:  "rollback",
		RollbackHost: rollback.NewHost(
			func() *checkpoint.Store { return cp },
			root,
			func() int { return 0 },
		),
	}, event.Discard)
	a.SetPreEditHook(func(ch diff.Change) { cp.Snapshot(ch) })

	err := a.Run(context.Background(), "break alpha and sign off")
	if !errors.Is(err, ErrVerifyExhausted) {
		t.Fatalf("Run err = %v, want ErrVerifyExhausted", err)
	}
	if !sessionHasUserMessageContaining(a.session, "## Rollback Preview") {
		t.Fatal("missing rollback preview in readiness retry")
	}
	if !sessionHasUserMessageContaining(a.session, "alpha.go") {
		t.Fatal("missing reverted path in rollback preview")
	}

	report := rollback.BuildReport(root, cp.RestorePlan(0))
	if len(report.Files) == 0 || report.Files[0].Removed == 0 {
		t.Fatalf("report=%+v", report)
	}
	notice := rollback.FormatAutoNotice(report)
	if !strings.Contains(notice, "Reverted changes") || !strings.Contains(notice, "alpha.go") {
		t.Fatalf("notice=%q", notice)
	}

	if _, _, err := cp.RestoreCode(0); err != nil {
		t.Fatalf("RestoreCode: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != good {
		t.Fatalf("file after restore = %q, want %q", got, good)
	}
}
