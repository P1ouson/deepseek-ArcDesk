package agent

import (
	"context"
	"testing"

	"arcdesk/internal/event"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
)

type reuseSink struct {
	events []event.Event
}

func (s *reuseSink) Emit(e event.Event) {
	s.events = append(s.events, e)
}

func TestToolReuseStatsOnExecuteBatch(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	reg.Add(fakeTool{name: "bash", readOnly: false})

	var sink reuseSink
	a := New(&scriptedProvider{name: "p", turns: nil}, reg, NewSession(""), Options{}, &sink)
	calls := []provider.ToolCall{
		{ID: "1", Name: "read_file", Arguments: `{"path":"a.go"}`},
		{ID: "2", Name: "read_file", Arguments: `{"path":"a.go"}`},
		{ID: "3", Name: "bash", Arguments: `{"command":"echo hi"}`},
	}
	a.executeBatch(context.Background(), calls)

	stats := a.ToolReuseStats()
	if stats.SessionCalls != 3 || stats.SessionDuplicates != 1 {
		t.Fatalf("session reuse = %+v, want 3 calls 1 duplicate", stats)
	}
	if stats.SessionCacheableCalls != 2 || stats.SessionCacheableDupes != 1 {
		t.Fatalf("cacheable reuse = %+v, want 2 calls 1 duplicate", stats)
	}
	dispatches := 0
	for _, e := range sink.events {
		if e.Kind == event.ToolDispatch {
			dispatches++
		}
	}
	if dispatches != 3 {
		t.Fatalf("dispatches = %d, want 3", dispatches)
	}
}

func TestToolReuseResetTurn(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	a := New(&scriptedProvider{name: "p", turns: nil}, reg, NewSession(""), Options{}, event.Discard)
	a.toolReuse.ResetTurn()
	a.executeBatch(context.Background(), []provider.ToolCall{
		{ID: "1", Name: "read_file", Arguments: `{"path":"x"}`},
	})
	if turn := a.toolReuse.Turn(); turn.Calls != 1 || turn.Duplicates != 0 {
		t.Fatalf("turn after first call = %+v", turn)
	}
	a.executeBatch(context.Background(), []provider.ToolCall{
		{ID: "2", Name: "read_file", Arguments: `{"path":"x"}`},
	})
	if turn := a.toolReuse.Turn(); turn.Calls != 2 || turn.Duplicates != 1 {
		t.Fatalf("turn after duplicate = %+v", turn)
	}
	a.toolReuse.ResetTurn()
	if turn := a.toolReuse.Turn(); turn.Calls != 0 {
		t.Fatalf("ResetTurn = %+v, want empty", turn)
	}
	if sess := a.toolReuse.Session(); sess.Calls != 2 || sess.Duplicates != 1 {
		t.Fatalf("session after ResetTurn = %+v", sess)
	}
}
