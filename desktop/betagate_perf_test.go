//go:build betagate

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type perfSample struct {
	Turn       int     `json:"turn"`
	ElapsedSec float64 `json:"elapsed_sec"`
	RSS_MB     float64 `json:"rss_mb"`
	Goroutines int     `json:"goroutines"`
	JSONL_KB   int64   `json:"jsonl_kb"`
	TurnSec    float64 `json:"turn_sec"`
}

func readRSSMB() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / (1024 * 1024)
}

func TestBetaGatePerfLongSoak(t *testing.T) {
	prompts := []string{
		"List files with glob in workspace; one line.",
		"Read the smallest .txt or .md file; reply OK_P1.",
		"Grep for OK_P in workspace; reply OK_P2.",
		"Reply OK_P3 only.",
		"Use glob for *.py; reply count.",
		"Read config or readme if any; reply OK_P4.",
		"Reply OK_P5.",
		"Glob **/*; reply top 3 names.",
		"Grep workspace for function; reply OK_P6.",
		"Reply OK_P7.",
		"Read two small files; reply OK_P8.",
		"Reply OK_P9.",
		"Use ls or glob; reply OK_P10.",
		"Reply OK_P11.",
		"Grep for import; reply OK_P12.",
		"Reply OK_P13.",
		"Read one file; reply OK_P14.",
		"Reply OK_P15.",
		"Glob *.json; reply OK_P16.",
		"Reply OK_P17.",
		"Grep error; reply OK_P18.",
		"Reply OK_P19.",
		"Summarize workspace in 2 sentences.",
		"Reply OK_P20.",
		"Read file; reply OK_P21.",
		"Reply OK_P22.",
		"Glob *; reply OK_P23.",
		"Reply OK_P24.",
		"Final: reply OK_P25_PERF_DONE.",
	}
	if len(prompts) < 25 {
		t.Fatal("need 25+ prompts")
	}

	rec := &recordingSink{}
	app := NewApp()
	tab := app.bootGlobalTabFromDisk(t, rec)
	defer closeTabController(tab)
	path := tab.Ctrl.SessionPath()

	start := time.Now()
	var samples []perfSample
	for i, p := range prompts {
		rec.reset()
		turnStart := time.Now()
		app.SubmitToTab(tab.ID, p)
		done := rec.waitTurnDone(t, 180*time.Second)
		if done.Err != nil {
			t.Fatalf("turn %d failed: %v", i+1, done.Err)
		}
		waitForAutosaveIdle(t, tab)
		var jsonlKB int64
		if fi, err := os.Stat(path); err == nil {
			jsonlKB = fi.Size() / 1024
		}
		turnSec := time.Since(turnStart).Seconds()
		samples = append(samples, perfSample{
			Turn:       i + 1,
			ElapsedSec: time.Since(start).Seconds(),
			RSS_MB:     readRSSMB(),
			Goroutines: runtime.NumGoroutine(),
			JSONL_KB:   jsonlKB,
			TurnSec:    turnSec,
		})
		if (i+1)%5 == 0 {
			t.Logf("perf turn %d: rss=%.1fMB goroutines=%d jsonl=%dKB turn=%.1fs",
				i+1, samples[len(samples)-1].RSS_MB, samples[len(samples)-1].Goroutines,
				samples[len(samples)-1].JSONL_KB, turnSec)
		}
	}

	outDir := filepath.Join("..", "benchmarks", "perf")
	_ = os.MkdirAll(outDir, 0o755)
	outPath := filepath.Join(outDir, "long_soak.json")
	payload := map[string]any{
		"turns":    len(prompts),
		"session":  path,
		"samples":  samples,
		"finished": time.Now().UTC().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	if err := os.WriteFile(outPath, b, 0o644); err != nil {
		t.Fatalf("write perf report: %v", err)
	}
	t.Logf("wrote %s", outPath)

	// Trend checks
	if len(samples) >= 10 {
		first5 := avgTurnSec(samples[:5])
		last5 := avgTurnSec(samples[len(samples)-5:])
		if last5 > first5*2.5 {
			t.Fatalf("turn latency degraded: first5 avg=%.1fs last5 avg=%.1fs", first5, last5)
		}
		rssStart := samples[4].RSS_MB
		rssEnd := samples[len(samples)-1].RSS_MB
		if rssEnd > rssStart+80 {
			t.Fatalf("heap growth suspicious: turn5=%.1fMB end=%.1fMB", rssStart, rssEnd)
		}
	}
}

func avgTurnSec(s []perfSample) float64 {
	if len(s) == 0 {
		return 0
	}
	var sum float64
	for _, x := range s {
		sum += x.TurnSec
	}
	return sum / float64(len(s))
}

func TestBetaGatePerfMultiTab(t *testing.T) {
	globalRoot := globalTabWorkspaceRoot()
	rec := &recordingSink{}
	app := NewApp()
	ensureWorkspace()

	makeTab := func(id string) *WorkspaceTab {
		tab := app.createTabEntryWithID("global", globalRoot, "", id)
		tab.sink = &tabEventSink{tabID: tab.ID, app: app, tap: rec}
		app.mu.Lock()
		app.tabs[tab.ID] = tab
		app.tabOrder = append(app.tabOrder, tab.ID)
		app.mu.Unlock()
		app.buildTabController(tab)
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			if tab.Ready && tab.Ctrl != nil {
				return tab
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatalf("tab %s not ready", id)
		return nil
	}

	t1 := makeTab("perf_tab_1")
	t2 := makeTab("perf_tab_2")
	t3 := makeTab("perf_tab_3")
	defer func() {
		closeTabController(t1)
		closeTabController(t2)
		closeTabController(t3)
	}()

	baseG := runtime.NumGoroutine()
	baseR := readRSSMB()

	for i, id := range []string{t1.ID, t2.ID, t3.ID, t1.ID, t2.ID} {
		rec.reset()
		app.mu.Lock()
		app.activeTabID = id
		app.mu.Unlock()
		app.SubmitToTab(id, fmt.Sprintf("Reply OK_TAB_%d only.", i+1))
		rec.waitTurnDone(t, 120*time.Second)
		waitForAutosaveIdle(t, app.tabs[id])
	}

	afterG := runtime.NumGoroutine()
	afterR := readRSSMB()
	t.Logf("multi-tab: goroutines %d -> %d (delta %d), rss %.1f -> %.1f MB",
		baseG, afterG, afterG-baseG, baseR, afterR)

	if afterG > baseG+200 {
		t.Fatalf("goroutine leak signal: +%d", afterG-baseG)
	}
}
