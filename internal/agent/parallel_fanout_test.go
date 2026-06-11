package agent

import (
	"sync/atomic"
	"testing"
	"time"

	"arcdesk/internal/event"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
)

func TestExploreParallelismScalesDownWithPromptSize(t *testing.T) {
	a := New(nil, tool.NewRegistry(), NewSession("sys"), Options{}, event.Discard)
	a.lastUsage.Store(&provider.Usage{PromptTokens: 60_000})
	if got := a.exploreParallelism(); got != 3 {
		t.Fatalf("60k prompt: got %d, want 3", got)
	}
	a.lastUsage.Store(&provider.Usage{PromptTokens: 120_000})
	if got := a.exploreParallelism(); got != 2 {
		t.Fatalf("120k prompt: got %d, want 2", got)
	}
	a.lastUsage.Store(nil)
	if got := a.exploreParallelism(); got != 4 {
		t.Fatalf("cold prompt: got %d, want 4", got)
	}
}

func TestRunParallelRespectsConcurrencyCap(t *testing.T) {
	var active atomic.Int32
	var peak atomic.Int32
	run := func(int) {
		cur := active.Add(1)
		for {
			old := peak.Load()
			if cur <= old || peak.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
	}
	runParallel(0, 6, 2, run)
	if peak.Load() > 2 {
		t.Fatalf("peak concurrency = %d, want <= 2", peak.Load())
	}
}
