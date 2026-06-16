package agent

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"arcdesk/internal/event"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
	"arcdesk/internal/toolcache"
	"arcdesk/internal/toolstats"
)

type cacheRunMetrics struct {
	dispatches      int
	executes        int32
	hits            int
	misses          int
	duplicates      int
	normalizedDupes int
}

func (m cacheRunMetrics) hitRate() float64 {
	d := m.hits + m.misses
	if d == 0 {
		return 0
	}
	return float64(m.hits) / float64(d)
}

func (m cacheRunMetrics) execReductionPct(baseline int32) float64 {
	if baseline <= 0 {
		return 0
	}
	return float64(baseline-m.executes) / float64(baseline) * 100
}

func runCacheWorkload(t *testing.T, workDir string, calls []provider.ToolCall, cacheOn bool, normalize bool) cacheRunMetrics {
	t.Helper()
	var execCalls int32
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true, calls: &execCalls})
	reg.Add(fakeTool{name: "grep", readOnly: true, calls: &execCalls})

	var shared *toolcache.Cache
	enabled := cacheOn
	norm := normalize
	opts := Options{
		ToolCacheEnabled:   &enabled,
		ToolCacheWorkDir:   workDir,
		ToolCacheNormalize: &norm,
	}
	if cacheOn {
		shared = toolcache.New()
		shared.SetKeyContext(toolstats.KeyContext{WorkDir: workDir, Normalize: normalize})
		opts.ToolCache = shared
	}
	a := New(&scriptedProvider{name: "metrics", turns: nil}, reg, NewSession(""), opts, event.Discard)
	a.executeBatch(context.Background(), calls)

	m := cacheRunMetrics{dispatches: len(calls), executes: atomic.LoadInt32(&execCalls)}
	if shared != nil {
		s := shared.Snapshot()
		m.hits = s.SessionHits
		m.misses = s.SessionMisses
	}
	reuse := a.ToolReuseStats()
	m.duplicates = reuse.SessionDuplicates
	m.normalizedDupes = reuse.SessionNormalizedDupes
	return m
}

// TestToolCacheMetricsAcceptance verifies cache-on materially reduces executes vs cache-off.
func TestToolCacheMetricsAcceptance(t *testing.T) {
	dir := t.TempDir()
	calls := make([]provider.ToolCall, 10)
	for i := range calls {
		calls[i] = provider.ToolCall{
			ID:        fmt.Sprintf("r%d", i+1),
			Name:      "read_file",
			Arguments: `{"path":"src/main.go"}`,
		}
	}

	off := runCacheWorkload(t, dir, calls, false, false)
	on := runCacheWorkload(t, dir, calls, true, false)

	if off.executes != int32(len(calls)) {
		t.Fatalf("cache-off executes = %d, want %d", off.executes, len(calls))
	}
	if on.executes != 1 {
		t.Fatalf("cache-on executes = %d, want 1", on.executes)
	}
	if on.hits != len(calls)-1 {
		t.Fatalf("cache-on hits = %d, want %d", on.hits, len(calls)-1)
	}

	reduction := on.execReductionPct(off.executes)
	if reduction < 80 {
		t.Fatalf("execute reduction = %.1f%%, want >= 80%%", reduction)
	}
	if rate := on.hitRate(); rate < 0.8 {
		t.Fatalf("cache hit rate = %.1f%%, want >= 80%%", rate*100)
	}

	t.Logf("Phase1 metrics: off_exec=%d on_exec=%d hits=%d hit_rate=%.1f%% exec_reduction=%.1f%%",
		off.executes, on.executes, on.hits, on.hitRate()*100, reduction)
}

// TestToolCacheP2NormalizationLift verifies Phase-2 normalization increases cache hits on alias args.
func TestToolCacheP2NormalizationLift(t *testing.T) {
	dir := t.TempDir()
	calls := []provider.ToolCall{
		{ID: "1", Name: "read_file", Arguments: `{"path":"main.go"}`},
		{ID: "2", Name: "read_file", Arguments: `{"path":"./main.go"}`},
		{ID: "3", Name: "read_file", Arguments: `{"path":"main.go","offset":0,"limit":0}`},
		{ID: "4", Name: "grep", Arguments: `{"pattern":"func","path":"."}`},
		{ID: "5", Name: "grep", Arguments: `{"pattern":"func"}`},
	}

	exact := runCacheWorkload(t, dir, calls, true, false)
	norm := runCacheWorkload(t, dir, calls, true, true)

	if exact.hits != 0 {
		t.Fatalf("exact-match cache hits = %d, want 0 on alias-only workload", exact.hits)
	}
	if exact.executes != int32(len(calls)) {
		t.Fatalf("exact-match executes = %d, want %d (no alias collapse)", exact.executes, len(calls))
	}

	if norm.executes > 2 {
		t.Fatalf("normalized executes = %d, want <= 2", norm.executes)
	}
	if norm.hits < 3 {
		t.Fatalf("normalized hits = %d, want >= 3", norm.hits)
	}
	if lift := norm.hits - exact.hits; lift < 3 {
		t.Fatalf("hit lift (P2-P1) = %d, want >= 3", lift)
	}

	reductionExact := exact.execReductionPct(int32(len(calls)))
	reductionNorm := norm.execReductionPct(int32(len(calls)))
	if delta := reductionNorm - reductionExact; delta < 40 {
		t.Fatalf("execute reduction lift = %.1fpp, want >= 40pp (exact=%.1f%% norm=%.1f%%)",
			delta, reductionExact, reductionNorm)
	}
	if norm.normalizedDupes < 2 {
		t.Fatalf("normalized duplicate detections = %d, want >= 2", norm.normalizedDupes)
	}

	t.Logf("Phase2 metrics: exact_exec=%d norm_exec=%d exact_hits=%d norm_hits=%d reduction_lift=%.1fpp norm_dupes=%d",
		exact.executes, norm.executes, exact.hits, norm.hits, reductionNorm-reductionExact, norm.normalizedDupes)
}
