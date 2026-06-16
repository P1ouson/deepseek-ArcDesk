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

func TestToolCacheNormalizePathAliases(t *testing.T) {
	dir := t.TempDir()
	var calls int32
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true, calls: &calls})

	shared := toolcache.New()
	normalize := true
	a := New(&scriptedProvider{name: "p", turns: nil}, reg, NewSession(""), Options{
		ToolCache:          shared,
		ToolCacheEnabled:   boolPtr(true),
		ToolCacheWorkDir:   dir,
		ToolCacheNormalize: &normalize,
	}, event.Discard)

	a.executeBatch(context.Background(), []provider.ToolCall{
		{ID: "1", Name: "read_file", Arguments: `{"path":"main.go"}`},
		{ID: "2", Name: "read_file", Arguments: `{"path":"./main.go"}`},
	})
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("Execute calls = %d, want 1 (path alias served from cache)", got)
	}
	stats := a.toolReuseStats()
	if stats.SessionNormalizedDupes != 1 {
		t.Fatalf("normalized dupes = %d, want 1", stats.SessionNormalizedDupes)
	}
}

func TestToolCacheNormalizeDisabledKeepsDistinctPaths(t *testing.T) {
	dir := t.TempDir()
	var calls int32
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true, calls: &calls})

	off := false
	a := New(&scriptedProvider{name: "p", turns: nil}, reg, NewSession(""), Options{
		ToolCacheEnabled:   boolPtr(true),
		ToolCacheWorkDir:   dir,
		ToolCacheNormalize: &off,
	}, event.Discard)

	a.executeBatch(context.Background(), []provider.ToolCall{
		{ID: "1", Name: "read_file", Arguments: `{"path":"main.go"}`},
		{ID: "2", Name: "read_file", Arguments: `{"path":"./main.go"}`},
	})
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("Execute calls = %d, want 2 when normalization disabled", got)
	}
}

func boolPtr(v bool) *bool { return &v }

func (a *Agent) toolReuseStats() event.ToolReuseStats { return a.ToolReuseStats() }
