package benchagent

import (
	"testing"
	"time"

	"arcdesk/internal/event"
	"arcdesk/internal/provider"
)

func TestCollectorRecordsTimingsAndReads(t *testing.T) {
	t.Setenv("BENCHMARK_AGENT", "1")
	ResetGlobal()
	c := Active()
	s := &Sink{C: c}

	c.MarkBootDone()

	time.Sleep(2 * time.Millisecond)
	s.Emit(event.Event{Kind: event.TurnStarted})
	s.Emit(event.Event{Kind: event.Reasoning, Text: "think"})
	s.Emit(event.Event{Kind: event.Text, Text: "hi"})
	s.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{Name: "read_file", Args: `{"path":"a.go","offset":0,"limit":250}`}})
	s.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{Name: "read_file", Args: `{"path":"a.go","offset":0,"limit":250}`, Output: "line\n"}})
	s.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{Name: "read_file", Args: `{"path":"a.go","offset":0,"limit":250}`}})
	s.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{Name: "read_file", Args: `{"path":"a.go","offset":250,"limit":250}`}})
	s.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{Name: "edit_file", Args: `{}`, ReadOnly: false}})
	s.Emit(event.Event{
		Kind: event.Usage,
		Usage: &provider.Usage{
			PromptTokens:     1000,
			CompletionTokens: 100,
			CacheHitTokens:   200,
			CacheMissTokens:  800,
		},
		Pricing: &provider.Pricing{CacheHit: 0.02, Input: 1, Output: 2, Currency: "CNY"},
	})
	s.Emit(event.Event{Kind: event.TurnDone})

	if c.totalToolCalls == 0 {
		t.Fatal("observe not recording tool dispatches")
	}
	RecordParallelBatch(3, 5)

	r := c.BuildReport(ProjectSize{Files: 10, LOC: 9000})
	if r.Timings.FirstActionMs == 0 && r.Timings.FirstReadMs == 0 {
		t.Fatalf("timings not recorded: %+v", r.Timings)
	}
	if r.ToolUsage.ReadFileCalls != 3 {
		t.Fatalf("read calls = %d, want 3", r.ToolUsage.ReadFileCalls)
	}
	if r.ReadPatterns.MaxPagingDepth < 1 {
		t.Fatalf("expected paging depth, got %+v", r.ReadPatterns)
	}
	if r.Fanout.MaxConcurrency != 3 {
		t.Fatalf("fanout max = %d, want 3", r.Fanout.MaxConcurrency)
	}
	if r.API.TotalAgentTurns != 1 {
		t.Fatalf("turns = %d, want 1", r.API.TotalAgentTurns)
	}
	if r.ToolReuse.DuplicateCalls != 1 {
		t.Fatalf("tool reuse duplicate calls = %d, want 1", r.ToolReuse.DuplicateCalls)
	}
	if r.ToolReuse.RepeatedKeys != 1 {
		t.Fatalf("tool reuse repeated keys = %d, want 1", r.ToolReuse.RepeatedKeys)
	}
}

func TestEnabled(t *testing.T) {
	t.Setenv("BENCHMARK_AGENT", "")
	if Enabled() {
		t.Fatal("expected disabled")
	}
	t.Setenv("BENCHMARK_AGENT", "1")
	if !Enabled() {
		t.Fatal("expected enabled")
	}
}
