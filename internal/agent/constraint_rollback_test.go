package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"arcdesk/internal/checkpoint"
	"arcdesk/internal/constraint"
	"arcdesk/internal/diff"
	"arcdesk/internal/event"
	"arcdesk/internal/evidence"
	"arcdesk/internal/instruction"
	"arcdesk/internal/provider"
	"arcdesk/internal/rollback"
	"arcdesk/internal/tool"
)

type previewWriter struct {
	name     string
	readOnly bool
	calls    *int32
}

func (p previewWriter) Name() string            { return p.name }
func (p previewWriter) Description() string     { return "" }
func (p previewWriter) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (p previewWriter) ReadOnly() bool          { return p.readOnly }

func (p previewWriter) Preview(args json.RawMessage) (diff.Change, error) {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	_ = json.Unmarshal(args, &in)
	kind := diff.Create
	oldText := ""
	if strings.Contains(in.Path, "App.css") {
		kind = diff.Modify
		oldText = "old"
	}
	return diff.Change{
		Path:    in.Path,
		Kind:    kind,
		OldText: oldText,
		NewText: in.Content,
	}, nil
}

func (p previewWriter) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	if p.calls != nil {
		atomic.AddInt32(p.calls, 1)
	}
	return "written", nil
}

func TestConstraintEngineBlocksWriter(t *testing.T) {
	root := t.TempDir()
	eng := constraint.NewEngine(&constraint.Host{Root: root}, constraint.Settings{
		BlockDuplicate: false,
		BlockFakeUI:    true,
		BlockArch:      false,
		AdviseReuse:    false,
	})
	var calls int32
	reg := tool.NewRegistry()
	reg.Add(previewWriter{name: "write_file", calls: &calls})
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{toolCallChunk("c1", "write_file", `{"path":"desktop/frontend/src/App.css","content":".x{display:none}"}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession(""), Options{ConstraintEngine: eng}, event.Discard)
	if err := a.Run(context.Background(), "edit css"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("write should be blocked, calls=%d", got)
	}
	last := lastToolResult(a.session, "write_file")
	if !strings.Contains(last, "blocked: [constraint]") {
		t.Fatalf("got %q", last)
	}
}

func TestConstraintEngineWarnsOnSuccessfulWrite(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "desktop", "frontend", "src", "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	eng := constraint.NewEngine(&constraint.Host{Root: root}, constraint.Settings{
		BlockDuplicate: false,
		BlockFakeUI:    false,
		BlockArch:      false,
		AdviseReuse:    true,
	})
	var calls int32
	reg := tool.NewRegistry()
	reg.Add(previewWriter{name: "write_file", calls: &calls})
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{toolCallChunk("c1", "write_file", `{"path":"desktop/frontend/src/utils.ts","content":"export function formatHelper(x:string){return x}"}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession(""), Options{ConstraintEngine: eng}, event.Discard)
	if err := a.Run(context.Background(), "add util"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls=%d", got)
	}
	last := lastToolResult(a.session, "write_file")
	if !strings.Contains(last, "[constraint]") {
		t.Fatalf("expected warn hint, got %q", last)
	}
}

func TestRollbackRetryContextOnFinalAttempt(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	cp := checkpoint.New("", root)
	cp.Begin(0, "fix", 0)
	cp.Snapshot(diff.Change{Path: "a.go", Kind: diff.Modify, OldText: "v0"})
	host := rollback.NewHost(func() *checkpoint.Store { return cp }, root, func() int { return 0 })

	a := New(nil, tool.NewRegistry(), NewSession(""), Options{
		ProjectChecks:    []instruction.VerifyCheck{{Command: "go test ./..."}},
		VerifyMaxRetries:         2,
		VerifyEnforceFinalAnswer: true,
		VerifyOnFailure:          "rollback",
		RollbackHost:     host,
	}, event.Discard)
	a.evidence = evidence.NewLedger()
	a.evidence.Record(evidence.ReceiptFromToolCall("write_file", json.RawMessage(`{"path":"a.go"}`), true, false))

	got := a.rollbackRetryContext(1)
	if !strings.Contains(got, "## Rollback Preview") {
		t.Fatalf("got %q", got)
	}
	if got := a.rollbackRetryContext(0); got != "" {
		t.Fatalf("early attempt should not preview rollback, got %q", got)
	}
}

func TestSetRollbackAndVerifyOnFailure(t *testing.T) {
	a := &Agent{}
	host := rollback.NewHost(nil, "", nil)
	a.SetRollbackHost(host)
	a.SetVerifyOnFailure("rollback")
	if a.rollbackHost == nil || a.verifyOnFailure != "rollback" {
		t.Fatalf("rollbackHost=%v onFailure=%q", a.rollbackHost, a.verifyOnFailure)
	}
}

func TestReadinessRetryOmitsRollbackPreviewWithoutFailedVerify(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	cp := checkpoint.New("", root)
	cp.Begin(0, "fix", 0)
	cp.Snapshot(diff.Change{Path: "a.go", Kind: diff.Modify, OldText: "v0"})
	host := rollback.NewHost(func() *checkpoint.Store { return cp }, root, func() int { return 0 })

	a := New(nil, tool.NewRegistry(), NewSession(""), Options{
		ProjectChecks:    []instruction.VerifyCheck{{Command: "go test ./..."}},
		VerifyMaxRetries:         2,
		VerifyEnforceFinalAnswer: true,
		VerifyOnFailure:          "rollback",
		RollbackHost:     host,
	}, event.Discard)
	a.evidence = evidence.NewLedger()
	a.evidence.Record(evidence.ReceiptFromToolCall("write_file", json.RawMessage(`{"path":"a.go"}`), true, false))

	msg := finalReadinessRetryMessage("run \"go test ./...\"")
	if strings.Contains(msg, "revert edits") {
		// defer/show guidance present
	} else {
		t.Fatalf("missing defer guidance: %q", msg)
	}
	if a.VerifyFailureShouldRollback() {
		t.Fatal("readiness without failed verify should not trigger rollback policy")
	}
	if got := a.rollbackRetryContext(1); got == "" || !strings.Contains(got, "## Rollback Preview") {
		t.Fatalf("rollbackRetryContext should still build preview for failed-verify path, got %q", got)
	}
	a.evidence.Record(evidence.Receipt{ToolName: "bash", Success: false, Command: "go test ./..."})
	if !a.VerifyFailureShouldRollback() {
		t.Fatal("failed verify should trigger rollback policy")
	}
}
