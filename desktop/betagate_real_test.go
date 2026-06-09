//go:build betagate

// Beta release gate against real AppData + live provider (same path as arcdesk-desktop.exe).
// Run: go test -tags=betagate -timeout 10m -run BetaGate -v .
package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"arcdesk/internal/config"
	"arcdesk/internal/event"
)

type recordingSink struct {
	mu  sync.Mutex
	evs []event.Event
}

func (r *recordingSink) Emit(e event.Event) {
	r.mu.Lock()
	r.evs = append(r.evs, e)
	r.mu.Unlock()
}

func (r *recordingSink) reset() {
	r.mu.Lock()
	r.evs = nil
	r.mu.Unlock()
}

func (r *recordingSink) waitTurnDone(t *testing.T, timeout time.Duration) event.Event {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		for _, e := range r.evs {
			if e.Kind == event.TurnDone {
				r.mu.Unlock()
				return e
			}
		}
		r.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("timed out waiting for TurnDone")
	return event.Event{}
}

func sessionHygiene() (jsonl, meta, orphan int) {
	dir := config.SessionDir()
	jsonls := map[string]struct{}{}
	for _, p := range globMust(filepath.Join(dir, "*.jsonl")) {
		jsonls[strings.TrimSuffix(filepath.Base(p), ".jsonl")] = struct{}{}
	}
	metas := globMust(filepath.Join(dir, "*.jsonl.meta"))
	orph := 0
	for _, m := range metas {
		base := strings.TrimSuffix(filepath.Base(m), ".meta")
		if _, ok := jsonls[strings.TrimSuffix(base, ".jsonl")]; !ok {
			orph++
		}
	}
	return len(jsonls), len(metas), orphan
}

func globMust(pattern string) []string {
	m, err := filepath.Glob(pattern)
	if err != nil {
		panic(err)
	}
	return m
}

func fileSize(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func (a *App) bootGlobalTabFromDisk(t *testing.T, rec *recordingSink) *WorkspaceTab {
	t.Helper()
	ensureWorkspace()
	f := loadTabsFile()
	if len(f.Tabs) == 0 {
		t.Fatal("desktop-tabs.json has no tabs")
	}
	entry := f.Tabs[0]
	a.mu.Lock()
	id := a.restoredTabIDLocked(entry.ID)
	a.mu.Unlock()
	tab := a.createTabEntryWithID("global", globalTabWorkspaceRoot(), entry.TopicID, id)
	tab.model = entry.Model
	tab.mode = persistedTabMode(entry.Mode)
	tab.sink = &tabEventSink{tabID: tab.ID, app: a, tap: rec}
	a.mu.Lock()
	a.tabs[tab.ID] = tab
	a.tabOrder = []string{tab.ID}
	a.activeTabID = tab.ID
	a.mu.Unlock()
	a.buildTabController(tab)
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if tab.Ready && tab.Ctrl != nil && tab.StartupErr == "" {
			return tab
		}
		if tab.Ready && tab.StartupErr != "" {
			t.Fatalf("tab startup error: %s", tab.StartupErr)
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timed out waiting for tab controller")
	return nil
}

func closeTabController(tab *WorkspaceTab) {
	if tab == nil || tab.Ctrl == nil {
		return
	}
	tab.Ctrl.Cancel()
	_ = tab.Ctrl.Snapshot()
	tab.Ctrl.Close()
	tab.Ctrl = nil
	tab.Ready = false
}

func TestBetaGateRestartResumeReal(t *testing.T) {
	j0, m0, o0 := sessionHygiene()
	t.Logf("hygiene before: jsonl=%d meta=%d orphan=%d", j0, m0, o0)

	marker := "BETAGATE_RESTART_" + time.Now().Format("20060102_150405")
	follow := marker + "_FOLLOWUP"

	rec := &recordingSink{}
	app := NewApp()
	tab := app.bootGlobalTabFromDisk(t, rec)
	path1 := tab.Ctrl.SessionPath()
	t.Logf("session path (boot): %s", path1)

	app.SubmitToTab(tab.ID, "Reply with exactly: "+marker)
	if done := rec.waitTurnDone(t, 120*time.Second); done.Err != nil {
		t.Fatalf("first turn failed: %v", done.Err)
	}
	waitForAutosaveIdle(t, tab)
	waitForFile(t, path1, marker)
	size1, _ := fileSize(path1)

	closeTabController(tab)
	rec.reset()

	tab2 := app.bootGlobalTabFromDisk(t, rec)
	defer closeTabController(tab2)
	path2 := tab2.Ctrl.SessionPath()
	t.Logf("session path (restart): %s", path2)

	if path2 != path1 {
		t.Fatalf("restart resumed different session: before=%q after=%q", path1, path2)
	}
	found := false
	for _, m := range tab2.Ctrl.History() {
		if strings.Contains(m.Content, marker) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("history after restart missing marker %q", marker)
	}

	rec.reset()
	app.SubmitToTab(tab2.ID, "Reply with exactly: "+follow)
	if done := rec.waitTurnDone(t, 120*time.Second); done.Err != nil {
		t.Fatalf("follow-up turn failed: %v", done.Err)
	}
	waitForAutosaveIdle(t, tab2)
	waitForFile(t, path2, follow)
	if path2 != tab2.Ctrl.SessionPath() {
		t.Fatalf("follow-up changed session path")
	}
	size2, _ := fileSize(path2)
	if size2 <= size1 {
		t.Fatalf("jsonl did not grow after follow-up: %d -> %d", size1, size2)
	}

	j1, m1, o1 := sessionHygiene()
	t.Logf("hygiene after: jsonl=%d meta=%d orphan=%d (delta orphan=%d)", j1, m1, o1, o1-o0)
}

func TestBetaGateInvalidAPIKeyRecovery(t *testing.T) {
	credPath := config.UserCredentialsPath()
	backup, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}
	t.Cleanup(func() { _ = os.WriteFile(credPath, backup, 0o600) })

	if err := os.WriteFile(credPath, []byte("DEEPSEEK_API_KEY=sk-invalid-betagate-key\n"), 0o600); err != nil {
		t.Fatalf("write bad credentials: %v", err)
	}
	t.Setenv("DEEPSEEK_API_KEY", "sk-invalid-betagate-key")

	rec := &recordingSink{}
	app := NewApp()
	tab := app.bootGlobalTabFromDisk(t, rec)
	defer closeTabController(tab)

	rec.reset()
	app.SubmitToTab(tab.ID, "hello")
	done := rec.waitTurnDone(t, 60*time.Second)
	if done.Err == nil {
		t.Fatal("invalid API key turn should fail with TurnDone error")
	}
	errMsg := done.Err.Error()
	if !strings.Contains(strings.ToLower(errMsg), "401") &&
		!strings.Contains(strings.ToLower(errMsg), "auth") &&
		!strings.Contains(strings.ToLower(errMsg), "api key") &&
		!strings.Contains(strings.ToLower(errMsg), "invalid") {
		t.Fatalf("unexpected error message for bad key: %q", errMsg)
	}
	t.Logf("bad key error (visible): %s", errMsg)

	_ = os.WriteFile(credPath, backup, 0o600)
	t.Setenv("DEEPSEEK_API_KEY", strings.TrimSpace(strings.TrimPrefix(string(backup), "DEEPSEEK_API_KEY=")))
	closeTabController(tab)
	rec.reset()

	tab2 := app.bootGlobalTabFromDisk(t, rec)
	defer closeTabController(tab2)
	rec.reset()
	app.SubmitToTab(tab2.ID, "Reply with exactly: BETAGATE_KEY_RECOVERED")
	done = rec.waitTurnDone(t, 120*time.Second)
	if done.Err != nil {
		t.Fatalf("turn after restoring key failed: %v", done.Err)
	}
	waitForFile(t, tab2.Ctrl.SessionPath(), "BETAGATE_KEY_RECOVERED")
}

func TestBetaGateMinimalSoakReal(t *testing.T) {
	prompts := []string{
		"List top-level files in the workspace using glob only; one sentence.",
		"Read the smallest text file you find; reply OK_SOAK_1.",
		"Use grep to search for SOAK in workspace; reply OK_SOAK_2.",
		"Reply OK_SOAK_3 with no tools.",
	}
	rec := &recordingSink{}
	app := NewApp()
	tab := app.bootGlobalTabFromDisk(t, rec)
	defer closeTabController(tab)

	stuck := 0
	for i, p := range prompts {
		rec.reset()
		app.SubmitToTab(tab.ID, p)
		done := rec.waitTurnDone(t, 120*time.Second)
		if done.Err != nil {
			t.Fatalf("soak turn %d failed: %v", i+1, done.Err)
		}
		waitForAutosaveIdle(t, tab)
	}
	path := tab.Ctrl.SessionPath()
	waitForFile(t, path, "OK_SOAK")
	if stuck > 0 {
		t.Fatalf("stuck turns: %d", stuck)
	}
	t.Logf("soak completed on session %s", path)
}
