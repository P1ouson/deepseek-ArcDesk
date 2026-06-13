package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/event"
	"arcdesk/internal/tool"
)

func TestNewHubZeroLimits(t *testing.T) {
	h := NewHub(Limits{MaxEntries: 0})
	if h.limits.MaxEntries != DefaultLimits().MaxEntries {
		t.Fatalf("limits = %+v", h.limits)
	}
}

func TestHubTurnAndIngestEdgeCases(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.SetTurn(4)
	if h.Turn() != 4 {
		t.Fatalf("turn = %d", h.Turn())
	}
	before := h.Stats().TotalEntries
	h.Ingest(KindConsole, "", "", "  ", nil)
	h.Ingest(KindConsole, "", "", "", map[string]string{})
	if h.Stats().TotalEntries != before {
		t.Fatal("empty ingest should no-op")
	}
	h.Ingest(KindConsole, "", "", "", map[string]string{"k": "v"})
	if h.Stats().TotalEntries != before+1 {
		t.Fatal("meta-only ingest should append")
	}
}

func TestHubSetStateEmptyKey(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.SetState("  ", "v")
	if len(h.State()) != 0 {
		t.Fatalf("state = %v", h.State())
	}
}

func TestHubTailLimitBounds(t *testing.T) {
	h := NewHub(DefaultLimits())
	for i := 0; i < 3; i++ {
		h.Ingest(KindConsole, LevelInfo, "", fmt.Sprintf("m%d", i), nil)
	}
	if got := len(h.Tail(TailQuery{Limit: 0})); got != 3 {
		t.Fatalf("default limit got %d", got)
	}
	for i := 0; i < 600; i++ {
		h.Ingest(KindConsole, LevelInfo, "", "x", nil)
	}
	if got := len(h.Tail(TailQuery{Limit: 1000})); got != 500 {
		t.Fatalf("capped limit got %d", got)
	}
}

func TestHubMergeStateBranches(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.Ingest(KindState, LevelInfo, "", "bare", nil)
	h.Ingest(KindState, LevelInfo, "", "snap", map[string]string{"": "skip", "online": "true"})
	state := h.State()
	if state["online"] != "true" || state[""] != "" {
		t.Fatalf("state = %v", state)
	}
}

func TestHubTrimRecomputesCounts(t *testing.T) {
	h := NewHub(Limits{MaxEntries: 2})
	h.Ingest(KindConsole, LevelError, "", "e1", nil)
	h.Ingest(KindConsole, LevelWarn, "", "w", nil)
	h.Ingest(KindConsole, LevelError, "", "e2", nil)
	stats := h.Stats()
	if stats.TotalEntries != 2 || stats.ErrorCount != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestIngestBatchNilHub(t *testing.T) {
	var h *Hub
	h.IngestBatch([]IngestItem{{Kind: KindConsole, Message: "x"}})
}

func TestCaptureSinkNilSafeAndShellTools(t *testing.T) {
	var s *CaptureSink
	s.Emit(event.Event{Kind: event.Notice, Text: "noop"})

	h := NewHub(DefaultLimits())
	s2 := NewCaptureSink(nil, h)
	s2.Emit(event.Event{Kind: event.Notice, Text: "   "})
	s2.Emit(event.Event{Kind: event.ToolProgress, Tool: event.Tool{Name: "run_shell", ID: "1", Output: "partial"}})
	s2.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{Name: "run_terminal_cmd", ID: "2", Output: "done"}})
	s2.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{Name: "other", ID: "3", Err: "boom"}})
	s2.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{Name: "bash", ID: "4", Output: "   ", Err: "fail"}})

	if h.Stats().TotalEntries < 4 {
		t.Fatalf("stats = %+v", h.Stats())
	}
}

func TestStderrWriterNilBranches(t *testing.T) {
	var sw *StderrWriter
	if n, err := sw.Write([]byte("x")); err != nil || n != 1 {
		t.Fatalf("nil writer n=%d err=%v", n, err)
	}
	w := NewStderrWriter(nil, NewHub(DefaultLimits()))
	if _, err := w.Write([]byte("line\n")); err != nil {
		t.Fatal(err)
	}
}

func TestIsVerifyCommandEmpty(t *testing.T) {
	if IsVerifyCommand("") || IsVerifyCommand("   ") {
		t.Fatal("empty command should not verify")
	}
}

func TestBuildVerifyContextEmptyHub(t *testing.T) {
	h := NewHub(DefaultLimits())
	if got := BuildVerifyContext(h, "go test ./...", ""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildVerifyContextRichObservations(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.Ingest(KindConsole, LevelError, "console", "TypeError", map[string]string{
		"url":    "http://localhost/app",
		"status": "500",
	})
	h.Ingest(KindGoLog, LevelWarn, "bash", "warn line", nil)
	for i := 0; i < 15; i++ {
		h.SetState(fmt.Sprintf("key%d", i), "v")
	}
	got := BuildVerifyContext(h, "pnpm build", "")
	if !strings.Contains(got, "http://localhost/app") || !strings.Contains(got, "[500]") {
		t.Fatalf("missing formatted tail: %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Fatal("expected truncated state keys")
	}
}

func TestFormatStateLinesEmpty(t *testing.T) {
	if FormatStateLines(nil) != nil {
		t.Fatal("nil state")
	}
	if FormatStateLines(map[string]string{}) != nil {
		t.Fatal("empty state")
	}
}

func TestRuntimeTailToolAdvancedFilters(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.Ingest(KindNetwork, LevelError, "fetch", "fail", nil)
	reg := tool.NewRegistry()
	RegisterTools(reg, h)
	tailTool, _ := reg.Get("runtime_tail")
	out, err := tailTool.Execute(context.Background(), json.RawMessage(`{
		"kind":"network",
		"errors_only":true,
		"since_seconds":3600,
		"limit":1
	}`))
	if err != nil || !strings.Contains(out, "runtime observation") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestRuntimeStateToolWithKeys(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.SetState("route", "/home")
	h.SetState("online", "true")
	reg := tool.NewRegistry()
	RegisterTools(reg, h)
	stateTool, _ := reg.Get("runtime_state")
	out, err := stateTool.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "route=/home") || strings.Contains(out, "empty") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestFormatTimeNonZero(t *testing.T) {
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := formatTime(ts); !strings.Contains(got, "2024-01-02") {
		t.Fatalf("got %q", got)
	}
}
