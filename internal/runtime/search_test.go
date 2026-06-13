package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestHubFindEmptyQuery(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.Ingest(KindConsole, LevelInfo, "console", "msg", nil)
	if got := h.Find(FindQuery{Query: "  ", Limit: 10}); len(got) != 0 {
		t.Fatalf("empty query should return nothing, got %+v", got)
	}
}

func TestHubFindKeyword(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.Ingest(KindConsole, LevelError, "console", "TypeError: undefined foo", nil)
	h.Ingest(KindGoLog, LevelInfo, "bash", "ok", nil)
	h.Ingest(KindNetwork, LevelWarn, "fetch", "timeout foo", nil)

	got := h.Find(FindQuery{Query: "foo", Limit: 10})
	if len(got) != 2 {
		t.Fatalf("find = %d entries, want 2", len(got))
	}
	if got[0].Message != "timeout foo" {
		t.Fatalf("newest first: %+v", got[0])
	}
}

func TestHubFindKindFilter(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.Ingest(KindConsole, LevelInfo, "console", "alpha", nil)
	h.Ingest(KindNetwork, LevelInfo, "net", "alpha", nil)
	got := h.Find(FindQuery{Query: "alpha", Kind: KindNetwork})
	if len(got) != 1 || got[0].Kind != KindNetwork {
		t.Fatalf("got %+v", got)
	}
}

func TestHubFindSourceMetaAndErrorsOnly(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.Ingest(KindConsole, LevelInfo, "my-source", "plain", nil)
	h.Ingest(KindConsole, LevelError, "other", "meta-hit", map[string]string{"trace": "needle"})
	got := h.Find(FindQuery{Query: "my-source", Limit: 5})
	if len(got) != 1 || got[0].Source != "my-source" {
		t.Fatalf("source match=%+v", got)
	}
	got = h.Find(FindQuery{Query: "needle", Limit: 5})
	if len(got) != 1 || got[0].Message != "meta-hit" {
		t.Fatalf("meta match=%+v", got)
	}
	h.Ingest(KindConsole, LevelInfo, "", "info-only", nil)
	got = h.Find(FindQuery{Query: "info", ErrorsOnly: true, Limit: 5})
	if len(got) != 0 {
		t.Fatalf("errors only=%+v", got)
	}
}

func TestRuntimeFindToolExecute(t *testing.T) {
	h := NewHub(DefaultLimits())
	h.Ingest(KindConsole, LevelError, "console", "panic: boom", nil)
	reg := tool.NewRegistry()
	RegisterTools(reg, h)
	findTool, ok := reg.Get("runtime_find")
	if !ok {
		t.Fatal("missing runtime_find")
	}
	if findTool.Description() == "" || !findTool.ReadOnly() {
		t.Fatal("find meta")
	}
	out, err := findTool.Execute(context.Background(), json.RawMessage(`{"query":"panic","limit":3}`))
	if err != nil || !strings.Contains(out, "panic") {
		t.Fatalf("find=%q err=%v", out, err)
	}
	out, err = findTool.Execute(context.Background(), json.RawMessage(`{"query":"missing"}`))
	if err != nil || !strings.Contains(out, "No runtime observations") {
		t.Fatalf("find empty=%q err=%v", out, err)
	}
	if _, err = findTool.Execute(context.Background(), json.RawMessage(`{"query":""}`)); err == nil {
		t.Fatal("empty query error")
	}
	if _, err = findTool.Execute(context.Background(), json.RawMessage(`{`)); err == nil {
		t.Fatal("invalid json")
	}
}
