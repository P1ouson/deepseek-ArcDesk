// toolcachebench runs a deterministic explore-style fixture twice (cache off vs on)
// and writes JSON reports under desktop/benchmarks/ for before/after comparison.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"arcdesk/internal/agent"
	"arcdesk/internal/benchagent"
	"arcdesk/internal/event"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
	"arcdesk/internal/tool/builtin"
	"arcdesk/internal/toolcache"
	"arcdesk/internal/toolstats"
)

type fixtureProvider struct {
	turn int
}

func (p *fixtureProvider) Name() string { return "toolcache-fixture" }

func (p *fixtureProvider) Stream(_ context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 16)
	switch p.turn {
	case 0:
		// Wave 1: parallel duplicate reads (typical explore fanout).
		ch <- toolCall("1", "read_file", `{"path":"main.go"}`)
		ch <- toolCall("2", "read_file", `{"path":"./main.go"}`)
		ch <- toolCall("3", "read_file", `{"path":"main.go"}`)
		ch <- toolCall("4", "read_file", `{"path":"lib.go"}`)
	case 1:
		ch <- toolCall("5", "grep", `{"pattern":"func ","path":"."}`)
		ch <- toolCall("6", "read_file", `{"path":"main.go"}`)
		ch <- toolCall("7", "read_file", `{"path":"lib.go"}`)
		ch <- toolCall("8", "read_file", `{"path":"README.md"}`)
	case 2:
		ch <- provider.Chunk{Type: provider.ChunkText, Text: "Explored fixture: entry main.go, helpers in lib.go."}
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

func toolCall(id, name, args string) provider.Chunk {
	return provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
		ID: id, Name: name, Arguments: args,
	}}
}

type summary struct {
	Variant             string  `json:"variant"`
	TotalToolCalls      int     `json:"totalToolCalls"`
	DuplicateCalls      int     `json:"duplicateCalls"`
	LocalCacheHits      int     `json:"localCacheHits"`
	LocalCacheMisses    int     `json:"localCacheMisses"`
	ExecuteReductionPct float64 `json:"executeReductionPct"`
	CachedResults       int     `json:"cachedResults"`
	ReportPath          string  `json:"reportPath"`
}

func main() {
	outDir := flag.String("out", "", "output directory (default desktop/benchmarks)")
	fixture := flag.String("fixture", "", "fixture project root")
	check := flag.Bool("check", false, "fail when cache-on does not beat cache-off thresholds")
	minHits := flag.Int("min-hits", 3, "with -check: minimum cache-on local hits")
	minReductionDelta := flag.Float64("min-reduction-delta", 40, "with -check: minimum execute-reduction delta (percentage points)")
	flag.Parse()

	repoRoot, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	absOut := *outDir
	if absOut == "" {
		absOut = filepath.Join(repoRoot, "desktop", "benchmarks")
	} else if !filepath.IsAbs(absOut) {
		absOut = filepath.Join(repoRoot, absOut)
	}
	absFixture := *fixture
	if absFixture == "" {
		absFixture = filepath.Join(repoRoot, "benchmarks", "fixtures", "toolcache")
	} else if !filepath.IsAbs(absFixture) {
		absFixture = filepath.Join(repoRoot, absFixture)
	}
	if st, err := os.Stat(absFixture); err != nil || !st.IsDir() {
		fatal(fmt.Errorf("fixture dir missing: %s", absFixture))
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil {
		fatal(err)
	}

	os.Setenv("BENCHMARK_AGENT", "1")
	prompt := "Explore this small Go project: find the entrypoint and helper functions. Read only what you need."

	var summaries []summary
	for _, variant := range []struct {
		name    string
		enabled bool
	}{
		{"cache-off", false},
		{"cache-on", true},
	} {
		path, rep, err := runVariant(variant.name, variant.enabled, absFixture, prompt, absOut)
		if err != nil {
			fatal(err)
		}
		summaries = append(summaries, summary{
			Variant:             variant.name,
			TotalToolCalls:      rep.ToolUsage.TotalToolCalls,
			DuplicateCalls:      rep.LocalToolCache.DuplicateCalls,
			LocalCacheHits:      rep.LocalToolCache.Hits,
			LocalCacheMisses:    rep.LocalToolCache.Misses,
			ExecuteReductionPct: rep.LocalToolCache.ExecuteReductionPct,
			CachedResults:       rep.LocalToolCache.CachedResults,
			ReportPath:          path,
		})
		fmt.Printf("wrote %s (%s hits=%d misses=%d reduction=%.1f%%)\n",
			path, variant.name, rep.LocalToolCache.Hits, rep.LocalToolCache.Misses, rep.LocalToolCache.ExecuteReductionPct)
	}

	off, on := summaries[0], summaries[1]
	delta := on.ExecuteReductionPct - off.ExecuteReductionPct
	fmt.Println()
	fmt.Println("=== toolcachebench summary ===")
	fmt.Printf("  tool dispatches     : off=%d on=%d\n", off.TotalToolCalls, on.TotalToolCalls)
	fmt.Printf("  duplicate calls     : off=%d on=%d\n", off.DuplicateCalls, on.DuplicateCalls)
	fmt.Printf("  local cache hits    : off=%d on=%d\n", off.LocalCacheHits, on.LocalCacheHits)
	fmt.Printf("  local cache misses  : off=%d on=%d\n", off.LocalCacheMisses, on.LocalCacheMisses)
	fmt.Printf("  execute reduction   : off=%.1f%% on=%.1f%% (delta +%.1fpp)\n",
		off.ExecuteReductionPct, on.ExecuteReductionPct, delta)
	fmt.Printf("  cached tool results : off=%d on=%d\n", off.CachedResults, on.CachedResults)

	sumPath := filepath.Join(absOut, "toolcache-summary.json")
	b, _ := json.MarshalIndent(map[string]any{
		"fixture": absFixture,
		"prompt":  prompt,
		"runs":    summaries,
		"delta": map[string]any{
			"executeReductionPct": delta,
			"cacheHits":           on.LocalCacheHits - off.LocalCacheHits,
		},
	}, "", "  ")
	if err := os.WriteFile(sumPath, b, 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("wrote %s\n", sumPath)

	if *check {
		if on.LocalCacheHits < *minHits {
			fatal(fmt.Errorf("acceptance failed: cache-on hits=%d want >= %d", on.LocalCacheHits, *minHits))
		}
		if delta < *minReductionDelta {
			fatal(fmt.Errorf("acceptance failed: reduction delta=%.1fpp want >= %.1fpp", delta, *minReductionDelta))
		}
		if on.LocalCacheHits <= off.LocalCacheHits {
			fatal(fmt.Errorf("acceptance failed: cache-on hits (%d) must exceed cache-off (%d)", on.LocalCacheHits, off.LocalCacheHits))
		}
		fmt.Println("acceptance check passed")
	}
}

func runVariant(variant string, cacheOn bool, fixtureDir, prompt, outDir string) (string, benchagent.Report, error) {
	benchagent.ResetGlobal()
	collector := benchagent.Active()
	if collector == nil {
		return "", benchagent.Report{}, fmt.Errorf("BENCHMARK_AGENT not enabled")
	}
	collector.SetMeta("toolcache-fixture", variant, fixtureDir, prompt)
	collector.MarkBootDone()

	reg := tool.NewRegistry()
	ws := builtin.Workspace{Dir: fixtureDir}
	for _, tl := range ws.Tools() {
		reg.Add(tl)
	}

	var shared *toolcache.Cache
	enabled := cacheOn
	normalize := true
	opts := agent.Options{
		ToolCacheEnabled:   &enabled,
		ToolCacheWorkDir:   fixtureDir,
		ToolCacheNormalize: &normalize,
	}
	if cacheOn {
		shared = toolcache.New()
		shared.SetKeyContext(toolstats.KeyContext{WorkDir: fixtureDir, Normalize: true})
		opts.ToolCache = shared
	}

	sink := &benchagent.Sink{C: collector, Inner: event.Discard}
	a := agent.New(&fixtureProvider{}, reg, agent.NewSession("benchmark"), opts, sink)
	if err := a.Run(context.Background(), prompt); err != nil {
		return "", benchagent.Report{}, err
	}

	projectSize := benchagent.ScanProjectSize(fixtureDir)
	report := collector.BuildReport(projectSize)
	path, err := benchagent.WriteReport(report, outDir)
	return path, report, err
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
