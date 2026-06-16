package agent

import (
	"context"
	"sync/atomic"
	"testing"

	"arcdesk/internal/event"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
	"arcdesk/internal/toolcache"
)

func TestToolCacheSkipsSecondExecute(t *testing.T) {
	var calls int32
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true, calls: &calls})

	shared := toolcache.New()
	on := true
	a := New(&scriptedProvider{name: "p", turns: nil}, reg, NewSession(""), Options{
		ToolCache: shared, ToolCacheEnabled: &on,
	}, event.Discard)

	callsArg := []provider.ToolCall{
		{ID: "1", Name: "read_file", Arguments: `{"path":"main.go"}`},
		{ID: "2", Name: "read_file", Arguments: `{"path":"main.go"}`},
	}
	a.executeBatch(context.Background(), callsArg)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("Execute calls = %d, want 1 (second served from cache)", got)
	}
	stats := a.toolCache.Snapshot()
	if stats.SessionHits != 1 || stats.SessionMisses != 1 {
		t.Fatalf("cache stats = %+v", stats)
	}
}

func TestToolCacheInvalidatesOnWrite(t *testing.T) {
	var readCalls int32
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true, calls: &readCalls})
	reg.Add(fakeTool{name: "write_file", readOnly: false})

	on := true
	a := New(&scriptedProvider{name: "p", turns: nil}, reg, NewSession(""), Options{ToolCacheEnabled: &on}, event.Discard)

	a.executeBatch(context.Background(), []provider.ToolCall{
		{ID: "1", Name: "read_file", Arguments: `{"path":"a.go"}`},
	})
	a.executeBatch(context.Background(), []provider.ToolCall{
		{ID: "2", Name: "write_file", Arguments: `{"path":"a.go","content":"x"}`},
	})
	a.executeBatch(context.Background(), []provider.ToolCall{
		{ID: "3", Name: "read_file", Arguments: `{"path":"a.go"}`},
	})

	if got := atomic.LoadInt32(&readCalls); got != 2 {
		t.Fatalf("read Execute calls = %d, want 2 after write invalidation", got)
	}
}

func TestToolCacheDisabledAlwaysExecutes(t *testing.T) {
	var calls int32
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "grep", readOnly: true, calls: &calls})

	off := false
	a := New(&scriptedProvider{name: "p", turns: nil}, reg, NewSession(""), Options{ToolCacheEnabled: &off}, event.Discard)
	a.executeBatch(context.Background(), []provider.ToolCall{
		{ID: "1", Name: "grep", Arguments: `{"pattern":"x"}`},
		{ID: "2", Name: "grep", Arguments: `{"pattern":"x"}`},
	})
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want 2 when cache disabled", got)
	}
}

func TestToolCacheHitMarksToolResult(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	var sink reuseSink
	on := true
	a := New(&scriptedProvider{name: "p", turns: nil}, reg, NewSession(""), Options{ToolCacheEnabled: &on}, &sink)
	a.executeBatch(context.Background(), []provider.ToolCall{
		{ID: "1", Name: "read_file", Arguments: `{"path":"x"}`},
		{ID: "2", Name: "read_file", Arguments: `{"path":"x"}`},
	})
	var cachedResults int
	for _, e := range sink.events {
		if e.Kind == event.ToolResult && e.Tool.Cached {
			cachedResults++
		}
	}
	if cachedResults != 1 {
		t.Fatalf("cached ToolResult events = %d, want 1", cachedResults)
	}
}
