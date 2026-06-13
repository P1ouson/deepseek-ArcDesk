package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/event"
	"arcdesk/internal/tool"
)

func TestHubIngestTailAndTrim(t *testing.T) {
	h := NewHub(Limits{MaxEntries: 3})
	h.SetTurn(2)
	h.Ingest(KindConsole, LevelInfo, "console", "a", nil)
	h.Ingest(KindNetwork, LevelError, "fetch", "fail", map[string]string{"url": "http://x", "status": "500"})
	h.Ingest(KindGoLog, LevelWarn, "bash", "warn", nil)
	h.Ingest(KindState, LevelInfo, "webview", "snap", map[string]string{"online": "true"})

	stats := h.Stats()
	if stats.TotalEntries != 3 {
		t.Fatalf("entries = %d, want trim to 3", stats.TotalEntries)
	}
	if stats.ByKind[KindNetwork] != 1 || stats.StateKeys == 0 {
		t.Fatalf("stats = %+v", stats)
	}
	tail := h.Tail(TailQuery{Kind: KindNetwork, Limit: 5})
	if len(tail) != 1 || tail[0].Level != LevelError {
		t.Fatalf("tail = %+v", tail)
	}
}

func TestHubTailSinceAndErrorsOnly(t *testing.T) {
	h := NewHub(DefaultLimits())
	past := time.Now().UTC().Add(-time.Hour)
	h.mu.Lock()
	h.entries = append(h.entries, Entry{ID: 1, Kind: KindConsole, Level: LevelError, At: past, Message: "old"})
	h.entries = append(h.entries, Entry{ID: 2, Kind: KindConsole, Level: LevelInfo, At: time.Now().UTC(), Message: "new"})
	h.mu.Unlock()

	got := h.Tail(TailQuery{Since: time.Now().UTC().Add(-5 * time.Minute), Errors: true, Limit: 10})
	if len(got) != 0 {
		t.Fatalf("expected no recent errors, got %+v", got)
	}
}

func TestCaptureSinkTapsShellAndNotice(t *testing.T) {
	h := NewHub(DefaultLimits())
	s := NewCaptureSink(event.Discard, h)
	s.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: "plugin warn"})
	s.Emit(event.Event{Kind: event.ToolProgress, Tool: event.Tool{Name: "bash", ID: "b1", Output: "partial"}})
	s.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{Name: "bash", ID: "b1", Output: "done", Err: "exit 1"}})
	stats := h.Stats()
	if stats.TotalEntries < 3 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestStderrWriterRecordsLines(t *testing.T) {
	h := NewHub(DefaultLimits())
	var buf bytes.Buffer
	w := NewStderrWriter(&buf, h)
	if _, err := w.Write([]byte("line1\nline2\n")); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "line1\nline2\n" {
		t.Fatalf("buf = %q", buf.String())
	}
	if h.Stats().ByKind[KindGoLog] != 2 {
		t.Fatalf("stats = %+v", h.Stats())
	}
}

func TestBuildVerifyContext(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.Ingest(KindConsole, LevelError, "console", "TypeError: x", nil)
	h.Ingest(KindNetwork, LevelError, "fetch", "GET /api 500", map[string]string{"url": "/api", "status": "500"})
	h.SetState("online", "false")
	got := BuildVerifyContext(h, "go test ./...", "stderr tail")
	if !strings.Contains(got, "## Runtime Observation") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "Console") || !strings.Contains(got, "Network") {
		t.Fatalf("missing sections: %q", got)
	}
	if BuildVerifyContext(nil, "go test", "x") != "" {
		t.Fatal("nil hub")
	}
	if BuildVerifyContext(h, "git status", "x") != "" {
		t.Fatal("non-verify cmd")
	}
}

func TestIsVerifyCommand(t *testing.T) {
	if !IsVerifyCommand("pnpm build") || !IsVerifyCommand("vitest run") {
		t.Fatal("expected verify")
	}
	if IsVerifyCommand("git status") {
		t.Fatal("non-verify")
	}
}

func TestIngestBatchAndFormatState(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.IngestBatch([]IngestItem{
		{Kind: KindConsole, Level: LevelInfo, Message: "hi"},
		{Kind: KindState, Level: LevelInfo, Message: "snap", Meta: map[string]string{"path": "/"}},
	})
	lines := FormatStateLines(h.State())
	if len(lines) != 1 || !strings.Contains(lines[0], "path=") {
		t.Fatalf("lines = %v", lines)
	}
}

func TestToolsExecute(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.Ingest(KindConsole, LevelInfo, "c", "hello", nil)
	reg := tool.NewRegistry()
	RegisterTools(reg, h)
	for _, name := range []string{"runtime_status", "runtime_tail", "runtime_state"} {
		tl, ok := reg.Get(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if tl.Name() == "" || tl.Description() == "" || !tl.ReadOnly() {
			t.Fatalf("tool meta empty for %s", name)
		}
	}
	statusTool, _ := reg.Get("runtime_status")
	status, err := statusTool.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(status, "Runtime hub") {
		t.Fatalf("status=%q err=%v", status, err)
	}
	tailTool, _ := reg.Get("runtime_tail")
	tail, err := tailTool.Execute(context.Background(), json.RawMessage(`{"kind":"console","level":"info","limit":5}`))
	if err != nil || !strings.Contains(tail, "hello") {
		t.Fatalf("tail=%q err=%v", tail, err)
	}
	emptyTail, err := tailTool.Execute(context.Background(), json.RawMessage(`{"kind":"network","limit":3}`))
	if err != nil || !strings.Contains(emptyTail, "No runtime observations") {
		t.Fatalf("emptyTail=%q err=%v", emptyTail, err)
	}
	stateTool, _ := reg.Get("runtime_state")
	stateOut, err := stateTool.Execute(context.Background(), nil)
	if err != nil || stateOut == "" {
		t.Fatalf("state=%q err=%v", stateOut, err)
	}
}

func TestBuildVerifyContextStderrFallback(t *testing.T) {
	h := NewHub(DefaultLimits())
	got := BuildVerifyContext(h, "go build ./...", strings.Repeat("x", 2000))
	if !strings.Contains(got, "Process output") {
		t.Fatalf("got %q", got)
	}
}

func TestCaptureSinkTurnStarted(t *testing.T) {
	h := NewHub(DefaultLimits())
	s := NewCaptureSink(nil, h)
	s.Emit(event.Event{Kind: event.TurnStarted})
	if h.Stats().ByKind[KindState] != 1 {
		t.Fatalf("stats = %+v", h.Stats())
	}
}

func TestStderrWriterPartialLine(t *testing.T) {
	h := NewHub(DefaultLimits())
	w := NewStderrWriter(io.Discard, h)
	_, _ = w.Write([]byte("partial"))
	if h.Stats().TotalEntries != 0 {
		t.Fatalf("stats = %+v", h.Stats())
	}
	_, _ = w.Write([]byte(" line\n"))
	if h.Stats().TotalEntries != 1 {
		t.Fatalf("stats = %+v", h.Stats())
	}
}

func TestHubTailLevelFilter(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.Ingest(KindConsole, LevelInfo, "", "info", nil)
	h.Ingest(KindConsole, LevelError, "", "err", nil)
	got := h.Tail(TailQuery{Level: LevelError, Limit: 5})
	if len(got) != 1 || got[0].Message != "err" {
		t.Fatalf("got %+v", got)
	}
}

func TestFormatTimeZero(t *testing.T) {
	if formatTime(time.Time{}) != "never" {
		t.Fatal("expected never")
	}
}

func TestResolvedLimits(t *testing.T) {
	if got := ResolvedLimits(0).MaxEntries; got != DefaultLimits().MaxEntries {
		t.Fatalf("got %d", got)
	}
	if got := ResolvedLimits(999999).MaxEntries; got != 65536 {
		t.Fatalf("cap got %d", got)
	}
}

func TestRegisterToolsNilSafe(t *testing.T) {
	RegisterTools(nil, NewHub(DefaultLimits()))
	RegisterTools(tool.NewRegistry(), nil)
}

func TestHubNilSafe(t *testing.T) {
	var h *Hub
	h.Ingest(KindConsole, LevelInfo, "", "x", nil)
	h.SetTurn(1)
	h.SetState("k", "v")
	if h.Tail(TailQuery{}) != nil || h.State() != nil || h.Stats().ByKind == nil {
		t.Fatal("nil hub methods should no-op")
	}
}
