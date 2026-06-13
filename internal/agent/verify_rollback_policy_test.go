package agent

import (
	"testing"

	"arcdesk/internal/evidence"
)

func TestVerifyFailureShouldRollback(t *testing.T) {
	a := &Agent{evidence: evidence.NewLedger()}
	if a.VerifyFailureShouldRollback() {
		t.Fatal("no writer should not rollback")
	}

	a.evidence.Record(evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"a.go"}})
	if a.VerifyFailureShouldRollback() {
		t.Fatal("readiness-only exhaustion should not rollback")
	}

	a.evidence.Record(evidence.Receipt{ToolName: "bash", Success: false, Command: "go test ./..."})
	if !a.VerifyFailureShouldRollback() {
		t.Fatal("failed verify should rollback when it is the last verify run")
	}

	a.evidence.Record(evidence.Receipt{ToolName: "bash", Success: true, Command: "go test ./..."})
	if a.VerifyFailureShouldRollback() {
		t.Fatal("recovered verify should not rollback")
	}
}

func TestVerifyFailureShouldRollbackCompoundSuccess(t *testing.T) {
	a := &Agent{evidence: evidence.NewLedger()}
	a.evidence.Record(evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"a.go"}})
	a.evidence.Record(evidence.Receipt{
		ToolName: "bash",
		Success:  true,
		Command:  `cd C:\demo; go test ./...`,
	})
	if a.VerifyFailureShouldRollback() {
		t.Fatal("successful compound verify should not trigger rollback")
	}
	if !a.evidence.HasSuccessfulCommandAfter("go test ./...", 0) {
		t.Fatal("compound go test should satisfy readiness")
	}
}
