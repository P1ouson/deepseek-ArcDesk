package knowledge

import (
	"context"
	"strings"
	"testing"

	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/failuremem"
)

func testRecordJudger(fix string) FuncCaptureJudger {
	return func(_ context.Context, in CaptureJudgeInput) (CaptureJudgment, error) {
		return CaptureJudgment{
			Record:    true,
			Signature: in.FailedCmd,
			Error:     truncateField(in.ErrOut, 200),
			Fix:       fix,
			Summary:   fix,
		}, nil
	}
}

func TestCaptureOnVerifyPass(t *testing.T) {
	root := t.TempDir()
	store, err := failuremem.Open(root, 50)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.KnowledgeConfig{}
	var emitted []event.Event
	sink := event.FuncSink(func(e event.Event) { emitted = append(emitted, e) })
	CaptureOnVerifyPass(context.Background(), testRecordJudger("Add or import Add in a.go, then re-run go test ./..."), store, cfg, sink,
		"go test ./...", "FAIL pkg\nundefined: Add", []string{"a.go"})
	if len(emitted) != 1 || emitted[0].Kind != event.KnowledgeCaptureRecorded {
		t.Fatalf("events = %v", emitted)
	}
	kc := emitted[0].KnowledgeCapture
	if kc.Signature != "go test ./..." || len(kc.Paths) != 1 {
		t.Fatalf("capture = %+v", kc)
	}
	if !strings.Contains(kc.Fix, "Add") {
		t.Fatalf("expected readable fix, got %q", kc.Fix)
	}
	entries, _ := store.List(10)
	if len(entries) != 1 || entries[0].Confidence != failuremem.ConfidenceDraft {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestCaptureOnVerifyPassSkipsWhenJudgeDeclines(t *testing.T) {
	root := t.TempDir()
	store, err := failuremem.Open(root, 50)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.KnowledgeConfig{}
	decline := FuncCaptureJudger(func(context.Context, CaptureJudgeInput) (CaptureJudgment, error) {
		return CaptureJudgment{Record: false, Reason: "demo noise"}, nil
	})
	var emitted []event.Event
	sink := event.FuncSink(func(e event.Event) { emitted = append(emitted, e) })
	CaptureOnVerifyPass(context.Background(), decline, store, cfg, sink, "go test ./...", "FAIL pkg\nundefined: Add", []string{"internal/counter/counter_test.go"})
	entries, _ := store.List(10)
	if len(entries) != 0 {
		t.Fatalf("expected no capture, got %d", len(entries))
	}
}

func TestCaptureOnVerifyPassHeuristicWhenJudgeDeclinesSourceFix(t *testing.T) {
	root := t.TempDir()
	store, err := failuremem.Open(root, 50)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.KnowledgeConfig{}
	decline := FuncCaptureJudger(func(context.Context, CaptureJudgeInput) (CaptureJudgment, error) {
		return CaptureJudgment{Record: false, Reason: "misread as noise"}, nil
	})
	var emitted []event.Event
	sink := event.FuncSink(func(e event.Event) { emitted = append(emitted, e) })
	CaptureOnVerifyPass(context.Background(), decline, store, cfg, sink,
		"go test -count=1 ./internal/counter/...",
		"--- FAIL: TestAdd\n    counter_test.go:8: Add(2) = 3\nFAIL",
		[]string{"internal/counter/counter.go"},
	)
	if len(emitted) != 1 || emitted[0].Kind != event.KnowledgeCaptureRecorded {
		t.Fatalf("events = %v", emitted)
	}
	entries, _ := store.List(10)
	if len(entries) != 1 {
		t.Fatalf("entries = %v", entries)
	}
}

func TestDismissCaptureBlocksAutoCapture(t *testing.T) {
	root := t.TempDir()
	store, err := failuremem.Open(root, 50)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.KnowledgeConfig{}
	judgment, err := testRecordJudger("fix it")(context.Background(), CaptureJudgeInput{
		FailedCmd: "go test ./...",
		ErrOut:    "FAIL pkg\nundefined: Add",
		Paths:     []string{"a.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, ok := proposalFromJudgment(judgment, []string{"a.go"})
	if !ok {
		t.Fatal("expected proposal")
	}
	if err := DismissCapture(root, proposal.Fingerprint); err != nil {
		t.Fatal(err)
	}
	var count int
	sink := event.FuncSink(func(e event.Event) {
		if e.Kind == event.KnowledgeCaptureRecorded {
			count++
		}
	})
	CaptureOnVerifyPass(context.Background(), testRecordJudger("fix"), store, cfg, sink, proposal.Signature, "FAIL pkg\nundefined: Add", proposal.Paths)
	if count != 0 {
		t.Fatalf("expected no auto capture after dismiss, got %d", count)
	}
}

func TestCaptureRequiresConfirmEmitsSuggestOnly(t *testing.T) {
	root := t.TempDir()
	store, err := failuremem.Open(root, 50)
	if err != nil {
		t.Fatal(err)
	}
	confirm := true
	cfg := config.KnowledgeConfig{RequireCaptureConfirm: &confirm}
	var emitted []event.Event
	sink := event.FuncSink(func(e event.Event) { emitted = append(emitted, e) })
	CaptureOnVerifyPass(context.Background(), testRecordJudger("fix undefined Add"), store, cfg, sink, "go test ./...", "FAIL pkg\nundefined: Add", []string{"a.go"})
	if len(emitted) != 1 || emitted[0].Kind != event.KnowledgeCaptureSuggest {
		t.Fatalf("events = %v", emitted)
	}
	entries, _ := store.List(10)
	if len(entries) != 0 {
		t.Fatalf("confirm mode should not write, got %d entries", len(entries))
	}
}

func TestJudgeCaptureRejectsDogfoodNoise(t *testing.T) {
	j, err := parseCaptureJudgment(`{"record":false,"reason":"trivial test literal flip"}`, CaptureJudgeInput{
		FailedCmd: "go test -count=1 ./internal/counter/...",
	})
	if err != nil {
		t.Fatal(err)
	}
	if j.Record {
		t.Fatal("expected record=false")
	}
	_, ok := proposalFromJudgment(j, []string{"internal/counter/counter_test.go"})
	if ok {
		t.Fatal("expected no proposal when judge declines")
	}
}

func TestProposalFromJudgmentReadableFix(t *testing.T) {
	proposal, ok := proposalFromJudgment(CaptureJudgment{
		Record:    true,
		Signature: "go test ./internal/counter/...",
		Error:     "undefined: Add in counter.go",
		Fix:       "Implement Add in counter.go or fix the import, then re-run go test.",
		Summary:   "Fix missing Add before running counter tests",
	}, []string{"internal/counter/counter.go"})
	if !ok {
		t.Fatal("expected proposal")
	}
	if proposal.Summary == "" || proposal.Fix == "" {
		t.Fatalf("proposal = %+v", proposal)
	}
}

func TestListViewDedupesByFingerprint(t *testing.T) {
	entries := []failuremem.Entry{
		{Signature: "go test ./pkg", Error: "fail", Fix: "fix one"},
		{Signature: "go test ./pkg", Error: "fail", Fix: "fix one merged"},
		{Signature: "go test ./other", Error: "fail", Fix: "fix two"},
	}
	got := ListView(entries, 10)
	if len(got) != 2 {
		t.Fatalf("ListView len = %d, want 2", len(got))
	}
}
