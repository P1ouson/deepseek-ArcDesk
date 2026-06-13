package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"arcdesk/internal/event"
	"arcdesk/internal/evidence"
	"arcdesk/internal/instruction"
	"arcdesk/internal/provider"
	"arcdesk/internal/selfdebug"
	"arcdesk/internal/tool"
	"arcdesk/internal/verification"
)

func TestSelfdebugImmediateHintOnBashFailure(t *testing.T) {
	tracker := selfdebug.NewTracker(selfdebug.Plan{
		Checks: []instruction.VerifyCheck{{Command: "go test ./..."}},
		Policy: verification.Policy{MaxRetries: 3},
	})
	writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"a.go"}}
	reg := tool.NewRegistry()
	reg.Add(&sequencedBashTool{failOutput: "FAIL"})
	a := New(&scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{toolCallChunk("w", "write_file", `{}`), toolCallChunk("b", "bash", `{"command":"go test ./..."}`), {Type: provider.ChunkDone}},
	}}, reg, NewSession(""), Options{
		ProjectChecks:    []instruction.VerifyCheck{{Command: "go test ./..."}},
		SelfdebugTracker: tracker,
	}, event.Discard)
	a.evidence = readinessLedger(writer)
	out := a.executeOne(context.Background(), provider.ToolCall{Name: "bash", Arguments: `{"command":"go test ./..."}`})
	if !strings.Contains(out.output, "[self-debug]") {
		t.Fatalf("output = %q", out.output)
	}
}

func TestSelfdebugRetryContextEndToEnd(t *testing.T) {
	tracker := selfdebug.NewTracker(selfdebug.Plan{
		Checks: []instruction.VerifyCheck{{Command: "go test ./...", SourcePath: "AGENTS.md"}},
		Policy: verification.Policy{MaxRetries: 3, OnFailure: "retry"},
	})
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	reg.Add(&sequencedBashTool{failOutput: "FAIL pkg"})

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("w1", "write_file", `{"path":"a.go","content":"x"}`),
			toolCallChunk("b1", "bash", `{"command":"go test ./..."}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
		{toolCallChunk("b2", "bash", `{"command":"go test ./..."}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "ok"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{
		ProjectChecks:    []instruction.VerifyCheck{{Command: "go test ./...", SourcePath: "AGENTS.md"}},
		SelfdebugTracker: tracker,
	}, event.Discard)

	if err := a.Run(context.Background(), "fix and verify"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !sessionHasUserMessageContaining(a.session, "## Self-debug Loop") {
		t.Fatal("missing self-debug block in readiness retry")
	}
	if !sessionHasUserMessageContaining(a.session, "go test ./...") {
		t.Fatal("missing failed command in retry")
	}
	snap := tracker.Snapshot()
	if snap.Phase != selfdebug.PhaseFailed && snap.Phase != selfdebug.PhaseVerify {
		t.Fatalf("tracker phase = %q", snap.Phase)
	}
}

func TestSelfdebugRetryContextNilTrackerUsesLegacy(t *testing.T) {
	writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"a.go"}}
	a := &Agent{
		evidence:      readinessLedger(writer),
		projectChecks: []instruction.VerifyCheck{{Command: "go test ./...", Category: "unit"}},
	}
	a.noteVerifyFailure(provider.ToolCall{Arguments: `{"command":"go test ./..."}`}, errors.New("fail"), "FAIL")
	got := a.verificationRetryContext()
	if !strings.Contains(got, "## Verification Engine") {
		t.Fatalf("got %q", got)
	}
	if got := a.selfdebugRetryContext(1); got != "" {
		t.Fatalf("expected empty without tracker, got %q", got)
	}
}
