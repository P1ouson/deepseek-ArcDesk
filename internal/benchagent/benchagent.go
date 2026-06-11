// Package benchagent provides dev-only benchmark instrumentation for agent
// exploration runs. Enable with BENCHMARK_AGENT=1; it is a no-op otherwise.
package benchagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Enabled reports whether benchmark instrumentation is active.
func Enabled() bool {
	v := strings.TrimSpace(os.Getenv("BENCHMARK_AGENT"))
	return v == "1" || strings.EqualFold(v, "true")
}

// Report is the JSON schema written to desktop/benchmarks/.
type Report struct {
	Label       string    `json:"label"`
	Variant     string    `json:"variant"`
	ProjectRoot string    `json:"projectRoot"`
	Prompt      string    `json:"prompt"`
	StartedAt   time.Time `json:"startedAt"`
	FinishedAt  time.Time `json:"finishedAt"`

	ProjectSize ProjectSize `json:"projectSize"`
	Timings     Timings     `json:"timings"`
	ToolUsage   ToolUsage   `json:"toolUsage"`
	ReadPatterns ReadPatterns `json:"readPatterns"`
	Cache       CacheStats  `json:"cache"`
	Fanout      FanoutStats `json:"fanout"`
	API         APIStats    `json:"api"`
	Error       string      `json:"error,omitempty"`
}

type ProjectSize struct {
	Files int `json:"files"`
	LOC   int `json:"loc"`
}

type Timings struct {
	ProjectOpenMs         int64 `json:"projectOpenMs"`
	FirstToolMs           int64 `json:"firstToolMs"`
	FirstReadMs           int64 `json:"firstReadMs"`
	FirstReasoningMs      int64 `json:"firstReasoningMs"`
	FirstAssistantTokenMs int64 `json:"firstAssistantTokenMs"`
	FirstActionMs         int64 `json:"firstActionMs"`
	TaskCompletedMs       int64 `json:"taskCompletedMs"`
}

type ToolUsage struct {
	TotalToolCalls       int     `json:"totalToolCalls"`
	ReadFileCalls        int     `json:"readFileCalls"`
	RepeatedReadFileCalls int    `json:"repeatedReadFileCalls"`
	AvgReadFileLines     float64 `json:"avgReadFileLines"`
	TruncatedReads       int     `json:"truncatedReads"`
}

type ReadPatterns struct {
	OffsetPagingChains int `json:"offsetPagingChains"`
	MaxPagingDepth     int `json:"maxPagingDepth"`
	DuplicateReads     int `json:"duplicateReads"`
}

type CacheStats struct {
	AvgHitRate         float64 `json:"avgHitRate"`
	LowestStepHitRate  float64 `json:"lowestStepHitRate"`
	HighestStepHitRate float64 `json:"highestStepHitRate"`
	PrefixChangedCount int     `json:"prefixChangedCount"`
}

type FanoutStats struct {
	AvgConcurrency  float64 `json:"avgConcurrency"`
	MaxConcurrency  int     `json:"maxConcurrency"`
	ThrottledRounds int     `json:"throttledRounds"`
}

type APIStats struct {
	TotalAgentTurns      int     `json:"totalAgentTurns"`
	TotalPromptTokens    int     `json:"totalPromptTokens"`
	TotalCompletionTokens int    `json:"totalCompletionTokens"`
	EstimatedCost        float64 `json:"estimatedCost"`
}

var (
	globalMu sync.Mutex
	global   *Collector
)

// Active returns the process-global collector when BENCHMARK_AGENT=1.
func Active() *Collector {
	if !Enabled() {
		return nil
	}
	globalMu.Lock()
	defer globalMu.Unlock()
	if global == nil {
		global = NewCollector()
	}
	return global
}

// ResetGlobal clears the process-global collector (tests).
func ResetGlobal() {
	globalMu.Lock()
	global = nil
	globalMu.Unlock()
}

// WriteReport writes r to desktop/benchmarks/benchmark-<timestamp>.json.
func WriteReport(r Report, outDir string) (string, error) {
	if outDir == "" {
		outDir = defaultOutDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("benchmark-%s.json", r.StartedAt.UTC().Format("20060102-150405"))
	path := filepath.Join(outDir, name)
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func defaultOutDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "desktop/benchmarks"
	}
	// Prefer repo-root/desktop/benchmarks when running from the monorepo.
	for _, base := range []string{cwd, filepath.Dir(cwd)} {
		candidate := filepath.Join(base, "desktop", "benchmarks")
		if st, err := os.Stat(filepath.Join(base, "desktop")); err == nil && st.IsDir() {
			return candidate
		}
	}
	return filepath.Join(cwd, "desktop", "benchmarks")
}
