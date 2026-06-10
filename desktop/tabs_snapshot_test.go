package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withCaptureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestTabSnapshotSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	a, tab := appWithTab(t, path)
	buf := withCaptureSlog(t)

	a.tabSnapshotLoop(tab)
	waitForAutosaveIdle(t, tab)
	waitForFile(t, path, "remember this turn")

	if strings.Contains(buf.String(), "tab snapshot failed") {
		t.Fatalf("unexpected warn on success: %q", buf.String())
	}
}

func TestTabSnapshotFailureLogsWarning(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "session.jsonl")
	if err := os.Mkdir(badPath, 0o755); err != nil {
		t.Fatal(err)
	}
	a, tab := appWithTab(t, badPath)
	buf := withCaptureSlog(t)

	a.tabSnapshotLoop(tab)
	waitForAutosaveIdle(t, tab)

	logs := buf.String()
	if !strings.Contains(logs, "tab snapshot failed") {
		t.Fatalf("expected snapshot warn log, got: %q", logs)
	}
	if !strings.Contains(logs, "test_tab") {
		t.Fatalf("expected tabID in warn log, got: %q", logs)
	}
}

func TestTabSnapshotFailureDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "session.jsonl")
	if err := os.Mkdir(badPath, 0o755); err != nil {
		t.Fatal(err)
	}
	a, tab := appWithTab(t, badPath)
	withCaptureSlog(t)

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("tabSnapshotLoop panicked: %v", r)
			}
		}()
		a.tabSnapshotLoop(tab)
	}()
	waitForAutosaveIdle(t, tab)
}
