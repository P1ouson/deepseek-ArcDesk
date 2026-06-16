package agent

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"arcdesk/internal/event"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
	"arcdesk/internal/toolcache"
)

type acceptanceReport struct {
	label          string
	toolDispatches int
	executeCalls   int32
	cacheHits      int
	cacheMisses    int
	duplicates     int
}

func runAcceptanceScenario(t *testing.T, label string, cacheOn bool, parallel bool, repeats int) acceptanceReport {
	t.Helper()
	var execCalls int32
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true, calls: &execCalls})

	var shared *toolcache.Cache
	enabled := cacheOn
	opts := Options{ToolCacheEnabled: &enabled}
	if cacheOn {
		shared = toolcache.New()
		opts.ToolCache = shared
	}
	a := New(&scriptedProvider{name: "acceptance", turns: nil}, reg, NewSession(""), opts, event.Discard)

	calls := make([]provider.ToolCall, repeats)
	for i := range calls {
		calls[i] = provider.ToolCall{
			ID:        fmt.Sprintf("r%d", i+1),
			Name:      "read_file",
			Arguments: `{"path":"src/main.go"}`,
		}
	}
	if parallel {
		a.executeBatch(context.Background(), calls)
	} else {
		for _, c := range calls {
			a.executeBatch(context.Background(), []provider.ToolCall{c})
		}
	}

	rep := acceptanceReport{
		label:          label,
		toolDispatches: repeats,
		executeCalls:   atomic.LoadInt32(&execCalls),
	}
	if shared != nil {
		s := shared.Snapshot()
		rep.cacheHits = s.SessionHits
		rep.cacheMisses = s.SessionMisses
	}
	reuse := a.ToolReuseStats()
	rep.duplicates = reuse.SessionDuplicates
	return rep
}

func pct(num, denom int) string {
	if denom <= 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", float64(num)/float64(denom)*100)
}

func execReduction(baseline, withCache int32) string {
	if baseline <= 0 {
		return "n/a"
	}
	saved := float64(baseline-withCache) / float64(baseline) * 100
	return fmt.Sprintf("%.1f%%", saved)
}

// TestToolCacheAcceptanceReport runs Phase-0/1 acceptance scenarios and prints
// a before/after style summary. Run with:
//
//	go test ./internal/agent -run TestToolCacheAcceptanceReport -v
func TestToolCacheAcceptanceReport(t *testing.T) {
	scenarios := []struct {
		off, on acceptanceReport
	}{
		{
			off: runAcceptanceScenario(t, "serial×5", false, false, 5),
			on:  runAcceptanceScenario(t, "serial×5", true, false, 5),
		},
		{
			off: runAcceptanceScenario(t, "parallel×5", false, true, 5),
			on:  runAcceptanceScenario(t, "parallel×5", true, true, 5),
		},
		{
			off: runAcceptanceScenario(t, "parallel×20", false, true, 20),
			on:  runAcceptanceScenario(t, "parallel×20", true, true, 20),
		},
	}

	var totalExecOff, totalExecOn int32
	var totalHits int
	for _, s := range scenarios {
		totalExecOff += s.off.executeCalls
		totalExecOn += s.on.executeCalls
		totalHits += s.on.cacheHits

		t.Logf("--- %s ---", s.off.label)
		t.Logf("  cache OFF: dispatches=%d executes=%d duplicates=%d",
			s.off.toolDispatches, s.off.executeCalls, s.off.duplicates)
		t.Logf("  cache ON : dispatches=%d executes=%d cache_hits=%d cache_misses=%d duplicates=%d hit_rate=%s exec_reduction=%s",
			s.on.toolDispatches, s.on.executeCalls, s.on.cacheHits, s.on.cacheMisses, s.on.duplicates,
			pct(s.on.cacheHits, s.on.toolDispatches), execReduction(s.off.executeCalls, s.on.executeCalls))

		if s.on.executeCalls != 1 {
			t.Errorf("%s: cache ON should execute once, got %d", s.on.label, s.on.executeCalls)
		}
		if s.on.cacheHits != s.on.toolDispatches-1 {
			t.Errorf("%s: cache hits = %d, want %d", s.on.label, s.on.cacheHits, s.on.toolDispatches-1)
		}
	}

	t.Log("=== Phase 1 acceptance summary ===")
	t.Logf("  total executes cache OFF : %d", totalExecOff)
	t.Logf("  total executes cache ON  : %d", totalExecOn)
	t.Logf("  total cache hits         : %d", totalHits)
	t.Logf("  execute reduction        : %s", execReduction(totalExecOff, totalExecOn))
	if reduction := float64(totalExecOff-totalExecOn) / float64(totalExecOff) * 100; reduction < 80 {
		t.Fatalf("aggregate execute reduction = %.1f%%, want >= 80%%", reduction)
	}
	t.Logf("  duplicate detection rate : %s (session duplicates / dispatches with cache ON)",
		pct(scenarios[0].on.duplicates+scenarios[1].on.duplicates+scenarios[2].on.duplicates,
			scenarios[0].on.toolDispatches+scenarios[1].on.toolDispatches+scenarios[2].on.toolDispatches))

	if os.Getenv("ACCEPTANCE_ONLY") == "1" {
		t.Log("ACCEPTANCE_ONLY=1 — metrics printed; assertions still enforced")
	}
}
