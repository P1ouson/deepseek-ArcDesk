package failuremem

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds failure memory search/record tools.
func RegisterTools(reg *tool.Registry, store *Store) {
	if reg == nil || store == nil {
		return
	}
	reg.Add(fmSearchTool{store: store})
	reg.Add(fmListTool{store: store})
	reg.Add(fmRecordTool{store: store})
}

type fmSearchTool struct{ store *Store }

func (fmSearchTool) Name() string { return "failuremem_search" }
func (fmSearchTool) Description() string {
	return "Search past failure→fix lessons by keyword (command output, error text, paths, tags). Use before debugging recurring issues."
}
func (fmSearchTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"}},"required":["query"]}`)
}
func (fmSearchTool) ReadOnly() bool { return true }
func (t fmSearchTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	entries, err := t.store.Search(p.Query, p.Limit)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(entries)
	if len(entries) == 0 {
		return fmt.Sprintf("No failure memory matched %q.\n[]", p.Query), nil
	}
	return fmt.Sprintf("%d match(es) for %q:\n%s", len(entries), p.Query, string(b)), nil
}

type fmListTool struct{ store *Store }

func (fmListTool) Name() string { return "failuremem_list" }
func (fmListTool) Description() string {
	return "List recent failure→fix lessons stored for this workspace."
}
func (fmListTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer"}}}`)
}
func (fmListTool) ReadOnly() bool { return true }
func (t fmListTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Limit int `json:"limit"`
	}
	if args != nil {
		_ = json.Unmarshal(args, &p)
	}
	entries, err := t.store.List(p.Limit)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(entries)
	if len(entries) == 0 {
		return "No failure memory entries yet.\n[]", nil
	}
	return fmt.Sprintf("%d recent entr(y/ies):\n%s", len(entries), string(b)), nil
}

type fmRecordTool struct{ store *Store }

func (fmRecordTool) Name() string { return "failuremem_record" }
func (fmRecordTool) Description() string {
	return "Record a failure signature, error excerpt, and fix for future reuse after resolving a bug or verify failure."
}
func (fmRecordTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"signature":{"type":"string","description":"Short label e.g. go test ./internal/foo"},"error":{"type":"string"},"fix":{"type":"string"},"paths":{"type":"array","items":{"type":"string"}},"tags":{"type":"array","items":{"type":"string"}}},"required":["signature","fix"]}`)
}
func (fmRecordTool) ReadOnly() bool { return false }
func (t fmRecordTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Signature string   `json:"signature"`
		Error     string   `json:"error"`
		Fix       string   `json:"fix"`
		Paths     []string `json:"paths"`
		Tags      []string `json:"tags"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	e := Entry{
		Signature: p.Signature,
		Error:     p.Error,
		Fix:       p.Fix,
		Paths:     append([]string(nil), p.Paths...),
		Tags:      append([]string(nil), p.Tags...),
	}
	if err := t.store.Record(e); err != nil {
		return "", err
	}
	return "Recorded failure memory entry for " + strings.TrimSpace(p.Signature), nil
}
