// summarizebench aggregates explorebench JSON reports into a markdown table.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"arcdesk/internal/benchagent"
)

func main() {
	dir := flag.String("dir", "desktop/benchmarks", "directory containing benchmark-*.json")
	out := flag.String("out", "", "write markdown report (default stdout)")
	flag.Parse()

	entries, err := filepath.Glob(filepath.Join(*dir, "benchmark-*.json"))
	if err != nil {
		fatal(err)
	}
	if len(entries) == 0 {
		fatal(fmt.Errorf("no reports in %s", *dir))
	}

	type key struct{ label, variant string }
	by := map[key]benchagent.Report{}
	for _, p := range entries {
		b, err := os.ReadFile(p)
		if err != nil {
			fatal(err)
		}
		var r benchagent.Report
		if err := json.Unmarshal(b, &r); err != nil {
			fatal(fmt.Errorf("%s: %w", p, err))
		}
		by[key{r.Label, r.Variant}] = r
	}

	labels := []string{"small", "medium", "large", "coding"}
	var sb strings.Builder
	sb.WriteString("## Benchmark Summary\n\n")
	sb.WriteString("| 指标 | Before | After | Delta |\n")
	sb.WriteString("| -- | --: | --: | --: |\n")

	row := func(name string, beforeFn, afterFn func(benchagent.Report) string) {
		_ = name
		_ = beforeFn
		_ = afterFn
	}

	_ = row
	for _, label := range labels {
		b, okB := by[key{label, "before"}]
		a, okA := by[key{label, "after"}]
		if !okB && !okA {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n### %s\n\n", label))
		sb.WriteString("| 指标 | Before | After | Delta |\n")
		sb.WriteString("| -- | --: | --: | --: |\n")
		writeRow := func(metric string, bv, av float64, unit string) {
			if !okB {
				bv = 0
			}
			if !okA {
				av = 0
			}
			delta := av - bv
			sb.WriteString(fmt.Sprintf("| %s | %.2f%s | %.2f%s | %+.2f%s |\n",
				metric, bv, unit, av, unit, delta, unit))
		}
		bTok, aTok := f(b, okB, func(r benchagent.Report) float64 { return float64(r.Timings.FirstAssistantTokenMs) }),
			f(a, okA, func(r benchagent.Report) float64 { return float64(r.Timings.FirstAssistantTokenMs) })
		writeRow("首次 assistant token (ms)", bTok, aTok, "")

		bAct, aAct := f(b, okB, func(r benchagent.Report) float64 { return float64(r.Timings.FirstActionMs) }),
			f(a, okA, func(r benchagent.Report) float64 { return float64(r.Timings.FirstActionMs) })
		writeRow("首次有效行动 (ms)", bAct, aAct, "")

		bHit, aHit := f(b, okB, func(r benchagent.Report) float64 { return r.Cache.AvgHitRate * 100 }),
			f(a, okA, func(r benchagent.Report) float64 { return r.Cache.AvgHitRate * 100 })
		writeRow("平均命中率 (%)", bHit, aHit, "")

		bSteps, aSteps := f(b, okB, func(r benchagent.Report) float64 { return float64(r.API.TotalAgentTurns) }),
			f(a, okA, func(r benchagent.Report) float64 { return float64(r.API.TotalAgentTurns) })
		writeRow("API steps", bSteps, aSteps, "")

		bRead, aRead := f(b, okB, func(r benchagent.Report) float64 { return float64(r.ToolUsage.ReadFileCalls) }),
			f(a, okA, func(r benchagent.Report) float64 { return float64(r.ToolUsage.ReadFileCalls) })
		writeRow("read_file 次数", bRead, aRead, "")

		bDepth, aDepth := f(b, okB, func(r benchagent.Report) float64 { return float64(r.ReadPatterns.MaxPagingDepth) }),
			f(a, okA, func(r benchagent.Report) float64 { return float64(r.ReadPatterns.MaxPagingDepth) })
		writeRow("最大分页深度", bDepth, aDepth, "")

		bConc, aConc := f(b, okB, func(r benchagent.Report) float64 { return float64(r.Fanout.MaxConcurrency) }),
			f(a, okA, func(r benchagent.Report) float64 { return float64(r.Fanout.MaxConcurrency) })
		writeRow("最大并发", bConc, aConc, "")

		bTotal, aTotal := f(b, okB, func(r benchagent.Report) float64 { return float64(r.Timings.TaskCompletedMs) }),
			f(a, okA, func(r benchagent.Report) float64 { return float64(r.Timings.TaskCompletedMs) })
		writeRow("总耗时 (ms)", bTotal, aTotal, "")

		if label == "coding" {
			sb.WriteString(fmt.Sprintf("\n- Before error: %q\n", strOr(b, okB, func(r benchagent.Report) string { return r.Error })))
			sb.WriteString(fmt.Sprintf("- After error: %q\n", strOr(a, okA, func(r benchagent.Report) string { return r.Error })))
		}
	}

	// Verdict section placeholder - filled manually or by heuristics
	sb.WriteString("\n### Verdict\n\n")

	keys := make([]key, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].label == keys[j].label {
			return keys[i].variant < keys[j].variant
		}
		return keys[i].label < keys[j].label
	})
	sb.WriteString("\n<details><summary>Raw reports</summary>\n\n")
	for _, k := range keys {
		r := by[k]
		sb.WriteString(fmt.Sprintf("- %s/%s: steps=%d reads=%d hit=%.1f%% total=%dms\n",
			k.label, k.variant, r.API.TotalAgentTurns, r.ToolUsage.ReadFileCalls,
			r.Cache.AvgHitRate*100, r.Timings.TaskCompletedMs))
	}
	sb.WriteString("\n</details>\n")

	outText := sb.String()
	if *out != "" {
		if err := os.WriteFile(*out, []byte(outText), 0o644); err != nil {
			fatal(err)
		}
		return
	}
	fmt.Print(outText)
}

func f(r benchagent.Report, ok bool, fn func(benchagent.Report) float64) float64 {
	if !ok {
		return 0
	}
	return fn(r)
}

func strOr(r benchagent.Report, ok bool, fn func(benchagent.Report) string) string {
	if !ok {
		return ""
	}
	return fn(r)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
