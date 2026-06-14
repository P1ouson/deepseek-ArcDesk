package knowledge

import (
	"context"
	"testing"

	"arcdesk/internal/provider"
)

type judgeMockProvider struct {
	text string
}

func (m *judgeMockProvider) Name() string { return "judge-mock" }

func (m *judgeMockProvider) Stream(_ context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 2)
	ch <- provider.Chunk{Type: provider.ChunkText, Text: m.text}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func TestProviderCaptureJudgerRecordsRealBug(t *testing.T) {
	prov := &judgeMockProvider{text: `{"record":true,"signature":"go test ./pkg","error":"undefined: Foo","fix":"Import or define Foo in pkg/foo.go before running tests.","summary":"Define Foo to fix go test ./pkg"}`}
	j := NewProviderCaptureJudger(prov)
	got, err := j.JudgeCapture(context.Background(), CaptureJudgeInput{
		FailedCmd: "go test ./pkg",
		ErrOut:    "FAIL\nundefined: Foo",
		Paths:     []string{"pkg/foo.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Record || got.Fix == "" {
		t.Fatalf("got = %+v", got)
	}
}

func TestProviderCaptureJudgerSkipsNoise(t *testing.T) {
	prov := &judgeMockProvider{text: `{"record":false,"reason":"only test expectation changed"}`}
	j := NewProviderCaptureJudger(prov)
	got, err := j.JudgeCapture(context.Background(), CaptureJudgeInput{
		FailedCmd: "go test -count=1 ./internal/counter/...",
		ErrOut:    "--- FAIL: TestAdd\n    counter_test.go:11: Add(3) = 5\nFAIL",
		Paths:     []string{"internal/counter/counter_test.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Record {
		t.Fatalf("expected skip, got %+v", got)
	}
}

func TestProviderCaptureJudgerAcceptsSourceLogicFix(t *testing.T) {
	prov := &judgeMockProvider{text: `{"record":true,"signature":"go test ./internal/counter/...","error":"Add returned state+1 so tests expected wrong totals","fix":"In counter.go Add, return state instead of state+1 so the running total matches accumulated adds.","summary":"Fix Add off-by-one return in counter.go"}`}
	j := NewProviderCaptureJudger(prov)
	got, err := j.JudgeCapture(context.Background(), CaptureJudgeInput{
		FailedCmd: "go test -count=1 ./internal/counter/...",
		ErrOut:    "--- FAIL: TestAdd\n    counter_test.go:8: Add(2) = 3\nFAIL",
		Paths:     []string{"internal/counter/counter.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Record || got.Fix == "" {
		t.Fatalf("expected record, got %+v", got)
	}
}

func TestPathsIncludeSourceFix(t *testing.T) {
	if !pathsIncludeSourceFix([]string{"internal/counter/counter.go"}) {
		t.Fatal("expected source fix")
	}
	if pathsIncludeSourceFix([]string{"internal/counter/counter_test.go"}) {
		t.Fatal("expected test-only paths to be false")
	}
}
