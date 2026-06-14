package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/evidence"
	"arcdesk/internal/failuremem"
	"arcdesk/internal/knowledge"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
)

func captureTestJudger() knowledge.FuncCaptureJudger {
	return knowledge.FuncCaptureJudger(func(_ context.Context, in knowledge.CaptureJudgeInput) (knowledge.CaptureJudgment, error) {
		return knowledge.CaptureJudgment{
			Record:    true,
			Signature: in.FailedCmd,
			Error:     in.ErrOut,
			Fix:       "Add or import Add in counter.go, then re-run go test -count=1 ./internal/counter/...",
			Summary:   "Fix missing Add in counter.go",
		}, nil
	})
}

func TestKnowledgeCaptureAutoAcrossTurns(t *testing.T) {
	root := t.TempDir()
	store, err := failuremem.Open(root, 50)
	if err != nil {
		t.Fatal(err)
	}
	var events []event.Event
	sink := event.FuncSink(func(e event.Event) { events = append(events, e) })

	bash := &sequencedBashTool{failOutput: "FAIL pkg\n./internal/counter/counter.go:10: undefined: Add\nexit status 1"}
	reg := tool.NewRegistry()
	reg.Add(bash)
	reg.Add(fakeTool{name: "write_file", readOnly: false})

	failTurn := &scriptedProvider{name: "fail", turns: [][]provider.Chunk{
		{toolCallChunk("b1", "bash", `{"command":"go test -count=1 ./internal/counter/..."}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "failed"}, {Type: provider.ChunkDone}},
	}}
	passTurn := &scriptedProvider{name: "pass", turns: [][]provider.Chunk{
		{toolCallChunk("w1", "write_file", `{"path":"internal/counter/counter.go","content":"fixed"}`), {Type: provider.ChunkDone}},
		{toolCallChunk("b2", "bash", `{"command":"go test -count=1 ./internal/counter/..."}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "ok"}, {Type: provider.ChunkDone}},
	}}

	a := New(failTurn, reg, NewSession(""), Options{
		FailureStore:  store,
		Knowledge:     config.KnowledgeConfig{},
		CaptureJudger: captureTestJudger(),
	}, sink)

	if err := a.Run(context.Background(), "run counter tests"); err != nil {
		t.Fatalf("fail turn Run: %v", err)
	}

	a.prov = passTurn
	if err := a.Run(context.Background(), "fix test and rerun"); err != nil {
		t.Fatalf("pass turn Run: %v", err)
	}

	waitForCaptureEvents(t, store, &events, 1)
	if count := captureRecordedCount(events); count != 1 {
		t.Fatalf("KnowledgeCaptureRecorded events = %d, want 1", count)
	}
}

func TestKnowledgeCaptureCrossTurnFixPaths(t *testing.T) {
	root := t.TempDir()
	store, err := failuremem.Open(root, 50)
	if err != nil {
		t.Fatal(err)
	}
	var events []event.Event
	sink := event.FuncSink(func(e event.Event) { events = append(events, e) })

	bash := &sequencedBashTool{failOutput: "--- FAIL: TestAdd\n    counter_test.go:8: Add(2) = 3\nFAIL"}
	reg := tool.NewRegistry()
	reg.Add(bash)
	reg.Add(fakeTool{name: "write_file", readOnly: false})

	failTurn := &scriptedProvider{name: "fail", turns: [][]provider.Chunk{
		{toolCallChunk("b1", "bash", `{"command":"go test -count=1 ./internal/counter/..."}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "failed"}, {Type: provider.ChunkDone}},
	}}
	fixTurn := &scriptedProvider{name: "fix", turns: [][]provider.Chunk{
		{toolCallChunk("w1", "write_file", `{"path":"internal/counter/counter.go","content":"fixed"}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "fixed"}, {Type: provider.ChunkDone}},
	}}
	passTurn := &scriptedProvider{name: "pass", turns: [][]provider.Chunk{
		{toolCallChunk("b2", "bash", `{"command":"go test -count=1 ./internal/counter/..."}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "ok"}, {Type: provider.ChunkDone}},
	}}

	decline := knowledge.FuncCaptureJudger(func(_ context.Context, in knowledge.CaptureJudgeInput) (knowledge.CaptureJudgment, error) {
		return knowledge.CaptureJudgment{Record: false, Reason: "misread"}, nil
	})

	a := New(failTurn, reg, NewSession(""), Options{
		FailureStore:  store,
		Knowledge:     config.KnowledgeConfig{},
		CaptureJudger: decline,
	}, sink)

	if err := a.Run(context.Background(), "run counter tests"); err != nil {
		t.Fatalf("fail turn Run: %v", err)
	}
	a.prov = fixTurn
	if err := a.Run(context.Background(), "fix counter.go only"); err != nil {
		t.Fatalf("fix turn Run: %v", err)
	}
	a.prov = passTurn
	if err := a.Run(context.Background(), "rerun tests"); err != nil {
		t.Fatalf("pass turn Run: %v", err)
	}

	waitForCaptureEvents(t, store, &events, 1)
	if count := captureRecordedCount(events); count != 1 {
		t.Fatalf("KnowledgeCaptureRecorded events = %d, want 1", count)
	}
}

func TestKnowledgeCaptureAutoSameTurn(t *testing.T) {
	root := t.TempDir()
	store, err := failuremem.Open(root, 50)
	if err != nil {
		t.Fatal(err)
	}
	var events []event.Event
	sink := event.FuncSink(func(e event.Event) { events = append(events, e) })

	reg := tool.NewRegistry()
	reg.Add(&sequencedBashTool{failOutput: "FAIL pkg\n./internal/counter/counter.go:10: undefined: Add\nexit status 1"})
	reg.Add(fakeTool{name: "write_file", readOnly: false})

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("w1", "write_file", `{"path":"internal/counter/counter.go","content":"fixed"}`),
			toolCallChunk("b1", "bash", `{"command":"go test -count=1 ./internal/counter/..."}`),
			{Type: provider.ChunkDone},
		},
		{toolCallChunk("b2", "bash", `{"command":"go test -count=1 ./internal/counter/..."}`), {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "ok"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{
		FailureStore:  store,
		Knowledge:     config.KnowledgeConfig{},
		CaptureJudger: captureTestJudger(),
	}, sink)

	if err := a.Run(context.Background(), "fix and verify"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	waitForCaptureEvents(t, store, &events, 1)
	if count := captureRecordedCount(events); count != 1 {
		t.Fatalf("KnowledgeCaptureRecorded events = %d, want 1", count)
	}
}

func TestNoteVerifyFailureAutoCaptures(t *testing.T) {
	root := t.TempDir()
	store, err := failuremem.Open(root, 50)
	if err != nil {
		t.Fatal(err)
	}
	var events []event.Event
	sink := event.FuncSink(func(e event.Event) { events = append(events, e) })

	a := New(nil, tool.NewRegistry(), NewSession(""), Options{
		FailureStore:  store,
		Knowledge:     config.KnowledgeConfig{},
		CaptureJudger: captureTestJudger(),
	}, sink)
	a.evidence = evidence.NewLedger()
	a.evidence.Record(evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"internal/counter/counter.go"}})

	a.noteVerifyFailure(
		context.Background(),
		provider.ToolCall{Arguments: `{"command":"go test -count=1 ./internal/counter/..."}`},
		errors.New("fail"),
		"FAIL pkg\n./internal/counter/counter.go:10: undefined: Add\nexit status 1",
	)
	a.noteVerifyFailure(
		context.Background(),
		provider.ToolCall{Arguments: `{"command":"go test -count=1 ./internal/counter/..."}`},
		nil,
		"ok",
	)

	waitForCaptureEvents(t, store, &events, 1)
	if count := captureRecordedCount(events); count != 1 {
		t.Fatalf("KnowledgeCaptureRecorded events = %d, want 1", count)
	}
}

func captureRecordedCount(events []event.Event) int {
	n := 0
	for _, e := range events {
		if e.Kind == event.KnowledgeCaptureRecorded {
			n++
		}
	}
	return n
}

func waitForCaptureEvents(t *testing.T, store *failuremem.Store, events *[]event.Event, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if captureRecordedCount(*events) >= want {
			entries, _ := store.List(10)
			if len(entries) >= want {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for capture: events=%d want=%d", captureRecordedCount(*events), want)
}
