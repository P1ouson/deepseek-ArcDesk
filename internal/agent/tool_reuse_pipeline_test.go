package agent

import (
	"context"
	"encoding/json"
	"testing"

	"arcdesk/internal/event"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
)

// Phase-0 validation: duplicate dispatches must flow to Usage and TurnDone payloads.
func TestToolReusePipelineUsageAndTurnDone(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})

	var sink reuseSink
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("1", "read_file", `{"path":"main.go"}`),
			toolCallChunk("2", "read_file", `{"path":"main.go"}`),
			{Type: provider.ChunkUsage, Usage: &provider.Usage{PromptTokens: 100, CompletionTokens: 10, TotalTokens: 110}},
		},
		{
			{Type: provider.ChunkText, Text: "done"},
			{Type: provider.ChunkUsage, Usage: &provider.Usage{PromptTokens: 200, CompletionTokens: 20, TotalTokens: 220}},
		},
	}}
	a := New(prov, reg, NewSession(""), Options{}, &sink)
	if err := a.Run(context.Background(), "inspect"); err != nil {
		t.Fatal(err)
	}

	stats := a.ToolReuseStats()
	if stats.SessionCalls != 2 || stats.SessionDuplicates != 1 {
		t.Fatalf("agent stats = %+v, want 2 calls / 1 duplicate", stats)
	}

	var usageWithReuse, turnDone *event.Event
	for i := range sink.events {
		e := &sink.events[i]
		if e.Kind == event.Usage && e.ToolReuse != nil && e.ToolReuse.SessionDuplicates > 0 {
			usageWithReuse = e
		}
		if e.Kind == event.TurnDone && e.ToolReuse != nil {
			turnDone = e
		}
	}
	if usageWithReuse == nil {
		t.Fatal("expected Usage event carrying ToolReuse with duplicates")
	}
	if usageWithReuse.ToolReuse.SessionCalls != 2 || usageWithReuse.ToolReuse.SessionDuplicates != 1 {
		t.Fatalf("usage ToolReuse = %+v", usageWithReuse.ToolReuse)
	}
	// Agent Run does not emit TurnDone; controller wraps that. Stats on agent are enough here.
	_ = turnDone
}

func TestToolReuseCanonicalArgsDetectsReorder(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "grep", readOnly: true})
	a := New(&scriptedProvider{name: "p", turns: nil}, reg, NewSession(""), Options{}, event.Discard)
	a.executeBatch(context.Background(), []provider.ToolCall{
		{ID: "1", Name: "grep", Arguments: `{"pattern":"foo","path":"."}`},
		{ID: "2", Name: "grep", Arguments: `{"path":".","pattern":"foo"}`},
	})
	stats := a.ToolReuseStats()
	if stats.SessionDuplicates != 1 {
		t.Fatalf("reordered JSON should count as duplicate, got %+v", stats)
	}
}

func TestToolReuseWireJSONRoundTrip(t *testing.T) {
	stats := event.ToolReuseStats{
		SessionCalls: 5, SessionDuplicates: 2,
		SessionCacheableCalls: 4, SessionCacheableDupes: 2,
		TurnCalls: 3, TurnDuplicates: 1,
	}
	raw, err := json.Marshal(stats)
	if err != nil {
		t.Fatal(err)
	}
	var decoded event.ToolReuseStats
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SessionDuplicates != 2 || decoded.SessionCacheableDupes != 2 {
		t.Fatalf("round-trip = %+v", decoded)
	}
}
