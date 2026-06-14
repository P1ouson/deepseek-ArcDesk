package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"arcdesk/internal/evidence"
	"arcdesk/internal/instruction"
	"arcdesk/internal/provider"
	"arcdesk/internal/runtime"
)

func TestRuntimeRetryContextWithVerifyFailure(t *testing.T) {
	hub := runtime.NewHub(runtime.DefaultLimits())
	hub.Ingest(runtime.KindGoLog, runtime.LevelError, "bash", "FAIL package", nil)
	writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"a.go"}}
	a := &Agent{
		evidence:      readinessLedger(writer),
		projectChecks: []instruction.VerifyCheck{{Command: "go test ./..."}},
		runtimeHub:    hub,
	}
	a.noteVerifyFailure(context.Background(),provider.ToolCall{Arguments: `{"command":"go test ./..."}`}, errors.New("fail"), "broken")
	got := a.runtimeRetryContext()
	if !strings.Contains(got, "## Runtime Observation") {
		t.Fatalf("got %q", got)
	}
}

func TestRuntimeRetryContextNilHub(t *testing.T) {
	a := &Agent{evidence: readinessLedger()}
	if got := a.runtimeRetryContext(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestRuntimeRetryContextWithoutWriterReceipt(t *testing.T) {
	hub := runtime.NewHub(runtime.DefaultLimits())
	hub.Ingest(runtime.KindConsole, runtime.LevelError, "console", "TypeError: pending", nil)
	a := &Agent{
		evidence:      evidence.NewLedger(),
		projectChecks: []instruction.VerifyCheck{{Command: "go test ./..."}},
		runtimeHub:    hub,
	}
	if got := a.runtimeRetryContext(); got == "" {
		t.Fatal("expected runtime context when verify is pending without writer receipt")
	}
}

func TestRuntimeRetryContextStderrFallback(t *testing.T) {
	hub := runtime.NewHub(runtime.DefaultLimits())
	writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"a.go"}}
	a := &Agent{
		evidence:      readinessLedger(writer),
		projectChecks: []instruction.VerifyCheck{{Command: "go build ./..."}},
		runtimeHub:    hub,
	}
	a.noteVerifyFailure(context.Background(),
		provider.ToolCall{Arguments: `{"command":"go build ./..."}`},
		errors.New("fail"),
		"compile error: undefined symbol",
	)
	got := a.runtimeRetryContext()
	if !strings.Contains(got, "Process output") || !strings.Contains(got, "compile error") {
		t.Fatalf("got %q", got)
	}
}
