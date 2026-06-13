package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"arcdesk/internal/tool"
)

// RegisterTools adds runtime observation tools to reg.
func RegisterTools(reg *tool.Registry, hub *Hub) {
	if reg == nil || hub == nil {
		return
	}
	reg.Add(rtStatusTool{hub: hub})
	reg.Add(rtTailTool{hub: hub})
	reg.Add(rtFindTool{hub: hub})
	reg.Add(rtStateTool{hub: hub})
}

type rtStatusTool struct{ hub *Hub }

func (rtStatusTool) Name() string { return "runtime_status" }
func (rtStatusTool) Description() string {
	return "Session runtime observation status: counts of console logs, shell output, network events, and tracked runtime state keys."
}
func (rtStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (rtStatusTool) ReadOnly() bool { return true }
func (t rtStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	stats := t.hub.Stats()
	b, _ := json.Marshal(stats)
	return fmt.Sprintf("Runtime hub: %d entries (%d errors), %d state keys, last activity %s\n%s",
		stats.TotalEntries, stats.ErrorCount, stats.StateKeys, formatTime(stats.LastActivityAt), string(b)), nil
}

type rtTailTool struct{ hub *Hub }

func (rtTailTool) Name() string { return "runtime_tail" }
func (rtTailTool) Description() string {
	return "Tail recent runtime observations (console, go_log, network, state). Newest first."
}
func (rtTailTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","enum":["console","go_log","wails","network","state"]},"level":{"type":"string","enum":["info","warn","error"]},"errors_only":{"type":"boolean"},"since_seconds":{"type":"integer"},"limit":{"type":"integer"}}}`)
}
func (rtTailTool) ReadOnly() bool { return true }
func (t rtTailTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Kind         string `json:"kind"`
		Level        string `json:"level"`
		ErrorsOnly   bool   `json:"errors_only"`
		SinceSeconds int    `json:"since_seconds"`
		Limit        int    `json:"limit"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &p)
	}
	q := TailQuery{Kind: Kind(p.Kind), Level: Level(p.Level), Limit: p.Limit, Errors: p.ErrorsOnly}
	if p.SinceSeconds > 0 {
		q.Since = time.Now().UTC().Add(-time.Duration(p.SinceSeconds) * time.Second)
	}
	entries := t.hub.Tail(q)
	b, _ := json.Marshal(entries)
	if len(entries) == 0 {
		return "No runtime observations matched.\n[]", nil
	}
	return fmt.Sprintf("%d runtime observation(s):\n%s", len(entries), string(b)), nil
}

type rtFindTool struct{ hub *Hub }

func (rtFindTool) Name() string { return "runtime_find" }
func (rtFindTool) Description() string {
	return "Search runtime observations (logs, shell, network) by keyword. Newest first."
}
func (rtFindTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"kind":{"type":"string","enum":["console","go_log","wails","network","state"]},"level":{"type":"string","enum":["info","warn","error"]},"errors_only":{"type":"boolean"},"limit":{"type":"integer"}},"required":["query"]}`)
}
func (rtFindTool) ReadOnly() bool { return true }
func (t rtFindTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Query      string `json:"query"`
		Kind       string `json:"kind"`
		Level      string `json:"level"`
		ErrorsOnly bool   `json:"errors_only"`
		Limit      int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if strings.TrimSpace(p.Query) == "" {
		return "", fmt.Errorf("query is required")
	}
	entries := t.hub.Find(FindQuery{
		Query:      p.Query,
		Kind:       Kind(p.Kind),
		Level:      Level(p.Level),
		Limit:      p.Limit,
		ErrorsOnly: p.ErrorsOnly,
	})
	b, _ := json.Marshal(entries)
	if len(entries) == 0 {
		return fmt.Sprintf("No runtime observations matched %q.\n[]", p.Query), nil
	}
	return fmt.Sprintf("%d match(es) for %q:\n%s", len(entries), p.Query, string(b)), nil
}

type rtStateTool struct{ hub *Hub }

func (rtStateTool) Name() string { return "runtime_state" }
func (rtStateTool) Description() string {
	return "Return the latest runtime state snapshot (route, connectivity, tab context, etc.)."
}
func (rtStateTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (rtStateTool) ReadOnly() bool { return true }
func (t rtStateTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	state := t.hub.State()
	b, _ := json.Marshal(state)
	if len(state) == 0 {
		return "Runtime state is empty.\n{}", nil
	}
	lines := FormatStateLines(state)
	return fmt.Sprintf("Runtime state (%d keys):\n%s\n%s", len(lines), strings.Join(lines, "\n"), string(b)), nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.UTC().Format(time.RFC3339)
}
