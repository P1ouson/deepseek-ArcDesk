package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"arcdesk/internal/benchagent"
	"arcdesk/internal/event"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
	"arcdesk/internal/tool/builtin"
	"arcdesk/internal/toolcache"
	"arcdesk/internal/toolstats"
)

type fixtureTurnProvider struct {
	turn int
}

func (p *fixtureTurnProvider) Name() string { return "fixture" }

func (p *fixtureTurnProvider) Stream(_ context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 16)
	switch p.turn {
	case 0:
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "1", Name: "read_file", Arguments: `{"path":"main.go"}`}}
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "2", Name: "read_file", Arguments: `{"path":"./main.go"}`}}
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "3", Name: "read_file", Arguments: `{"path":"main.go"}`}}
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "4", Name: "read_file", Arguments: `{"path":"lib.go"}`}}
	case 1:
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "5", Name: "grep", Arguments: `{"pattern":"func ","path":"."}`}}
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "6", Name: "read_file", Arguments: `{"path":"main.go"}`}}
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "7", Name: "read_file", Arguments: `{"path":"lib.go"}`}}
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "8", Name: "read_file", Arguments: `{"path":"README.md"}`}}
	default:
		ch <- provider.Chunk{Type: provider.ChunkText, Text: "done"}
	}
	ch <- provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{
		PromptTokens: 500, CompletionTokens: 50, TotalTokens: 550,
	}}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	p.turn++
	close(ch)
	return ch, nil
}

func runFixtureVariant(t *testing.T, fixtureDir string, cacheOn bool) benchagent.Report {
	t.Helper()
	t.Setenv("BENCHMARK_AGENT", "1")
	benchagent.ResetGlobal()
	collector := benchagent.Active()
	if collector == nil {
		t.Fatal("benchagent not enabled")
	}
	collector.SetMeta("toolcache-fixture", "test", fixtureDir, "fixture")
	collector.MarkBootDone()

	reg := tool.NewRegistry()
	ws := builtin.Workspace{Dir: fixtureDir}
	for _, tl := range ws.Tools() {
		reg.Add(tl)
	}

	normalize := true
	enabled := cacheOn
	opts := Options{
		ToolCacheEnabled:   &enabled,
		ToolCacheWorkDir:   fixtureDir,
		ToolCacheNormalize: &normalize,
	}
	if cacheOn {
		shared := toolcache.New()
		shared.SetKeyContext(toolstats.KeyContext{WorkDir: fixtureDir, Normalize: true})
		opts.ToolCache = shared
	}

	sink := &benchagent.Sink{C: collector, Inner: event.Discard}
	a := New(&fixtureTurnProvider{}, reg, NewSession("bench"), opts, sink)
	if err := a.Run(context.Background(), "explore"); err != nil {
		t.Fatal(err)
	}
	return collector.BuildReport(benchagent.ScanProjectSize(fixtureDir))
}

func toolcacheFixtureDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "benchmarks", "fixtures", "toolcache")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("benchmarks/fixtures/toolcache not found from test cwd")
		}
		dir = parent
	}
}

// TestToolCacheFixtureMetricsAcceptance runs the deterministic fixture and
// requires cache-on to materially beat cache-off on local cache hits.
func TestToolCacheFixtureMetricsAcceptance(t *testing.T) {
	fixture := toolcacheFixtureDir(t)

	off := runFixtureVariant(t, fixture, false)
	on := runFixtureVariant(t, fixture, true)

	if off.LocalToolCache.Hits != 0 {
		t.Fatalf("cache-off hits = %d, want 0", off.LocalToolCache.Hits)
	}
	if on.LocalToolCache.Hits < 3 {
		t.Fatalf("cache-on hits = %d, want >= 3", on.LocalToolCache.Hits)
	}
	delta := on.LocalToolCache.ExecuteReductionPct - off.LocalToolCache.ExecuteReductionPct
	if delta < 40 {
		t.Fatalf("execute reduction delta = %.1fpp, want >= 40pp (off=%.1f%% on=%.1f%%)",
			delta, off.LocalToolCache.ExecuteReductionPct, on.LocalToolCache.ExecuteReductionPct)
	}
	if on.LocalToolCache.HitRate < 0.4 {
		t.Fatalf("cache-on hit rate = %.1f%%, want >= 40%%", on.LocalToolCache.HitRate*100)
	}

	t.Logf("fixture metrics: off_hits=%d on_hits=%d on_hit_rate=%.1f%% reduction_delta=%.1fpp cached=%d",
		off.LocalToolCache.Hits, on.LocalToolCache.Hits, on.LocalToolCache.HitRate*100,
		delta, on.LocalToolCache.CachedResults)
}
