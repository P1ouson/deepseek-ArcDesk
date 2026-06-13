package agent

import (
	"errors"
	"strings"
	"testing"

	"arcdesk/internal/evidence"
	"arcdesk/internal/instruction"
	"arcdesk/internal/provider"
)

func TestVerificationRetryContextInjected(t *testing.T) {
	writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"a.go"}}
	a := &Agent{
		evidence: readinessLedger(writer),
		projectChecks: []instruction.VerifyCheck{
			{Command: "go test ./...", Category: "unit"},
			{Command: "pnpm exec playwright test --reporter=line", Category: "e2e"},
		},
	}
	a.noteVerifyFailure(provider.ToolCall{Arguments: `{"command":"go test ./..."}`}, errors.New("fail"), "FAIL pkg")
	got := a.verificationRetryContext()
	if !strings.Contains(got, "## Verification Engine") {
		t.Fatalf("got %q", got)
	}
}

func TestVerificationRetryContextNilChecks(t *testing.T) {
	a := &Agent{evidence: readinessLedger()}
	if got := a.verificationRetryContext(); got != "" {
		t.Fatalf("got %q", got)
	}
}
