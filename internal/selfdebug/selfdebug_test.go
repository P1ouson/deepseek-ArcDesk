package selfdebug

import (
	"strings"
	"testing"

	"arcdesk/internal/instruction"
	"arcdesk/internal/verification"
)

func TestBuildImmediateHint(t *testing.T) {
	in := Input{
		HasWriter: true,
		FailedCmd: "go test ./...",
		Checks: []instruction.VerifyCheck{
			{Command: "go test ./..."},
			{Command: "go vet ./..."},
		},
		WriterIndex: 1,
		HasSuccessfulCommandAfter: func(string, int) bool { return false },
	}
	got := BuildImmediateHint(in)
	if !strings.Contains(got, "[self-debug]") || !strings.Contains(got, "go test ./...") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "go vet ./...") {
		t.Fatalf("missing other pending check: %q", got)
	}
}

func TestBuildRetryContext(t *testing.T) {
	in := Input{
		HasWriter:    true,
		FailedCmd:    "go test ./...",
		Stderr:       "FAIL pkg",
		WrittenPaths: []string{"a.go"},
		Checks: []instruction.VerifyCheck{
			{Command: "go test ./...", Category: string(verification.CategoryUnit)},
		},
		Attempt:    2,
		MaxRetries: 3,
		HasSuccessfulCommandAfter: func(string, int) bool { return false },
	}
	got := BuildRetryContext(in)
	for _, want := range []string{"## Self-debug Loop", "write → verify", "Attempt 2/3", "a.go", "## Verification Engine"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestAnalyzeSnapshot(t *testing.T) {
	in := Input{
		HasWriter: true,
		FailedCmd: "go build ./...",
		Checks: []instruction.VerifyCheck{
			{Command: "go build ./..."},
			{Command: "go test ./..."},
		},
		WriterIndex: 0,
		HasSuccessfulCommandAfter: func(cmd string, _ int) bool {
			return cmd == "go test ./..."
		},
	}
	snap := AnalyzeSnapshot(in)
	if snap.Phase != PhaseFailed || len(snap.PendingChecks) != 1 || snap.PassedChecks[0] != "go test ./..." {
		t.Fatalf("snap = %+v", snap)
	}
}

func TestSelfdebugTool(t *testing.T) {
	tracker := NewTracker(Plan{
		Checks: []instruction.VerifyCheck{{Command: "go test ./..."}},
		Policy: verification.Policy{MaxRetries: 3, OnFailure: "retry"},
	})
	tracker.Update(Snapshot{Phase: PhaseFailed, FailedCmd: "go test ./...", Attempt: 1, MaxRetries: 3})
	reg := newTestRegistry(t)
	RegisterTools(reg, tracker)
	tool, ok := reg.Get("selfdebug_status")
	if !ok {
		t.Fatal("missing tool")
	}
	out, err := tool.Execute(t.Context(), nil)
	if err != nil || !strings.Contains(out, "phase=failed") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestIsVerifyCommand(t *testing.T) {
	if !IsVerifyCommand("pnpm exec vitest run") || IsVerifyCommand("echo hi") {
		t.Fatal("IsVerifyCommand mismatch")
	}
}

func TestBuildRetryContextNoWriter(t *testing.T) {
	if got := BuildRetryContext(Input{}); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestRegisterToolsNil(t *testing.T) {
	RegisterTools(nil, NewTracker(Plan{}))
}

func TestTrackerNilSafe(t *testing.T) {
	var t0 *Tracker
	t0.Update(Snapshot{})
	if t0.Snapshot().Phase != "" {
		t.Fatal("nil snapshot")
	}
}
