package uirag

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds ui_* discovery tools.
func RegisterTools(reg *tool.Registry, idx *Index) {
	if reg == nil || idx == nil {
		return
	}
	reg.Add(uiStatusTool{idx: idx})
	reg.Add(uiListTool{idx: idx})
	reg.Add(uiFindTool{idx: idx})
	reg.Add(uiReadTool{idx: idx})
}

type uiStatusTool struct{ idx *Index }

func (uiStatusTool) Name() string { return "ui_status" }
func (uiStatusTool) Description() string {
	return "UI component index status: scan roots and discovered React/TSX export count."
}
func (uiStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (uiStatusTool) ReadOnly() bool { return true }
func (t uiStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	return fmt.Sprintf("UI index: %d component(s) across %d scan root(s)\nscan_roots: %s",
		len(t.idx.Components), len(t.idx.ScanRoots), strings.Join(t.idx.ScanRoots, ", ")), nil
}

type uiListTool struct{ idx *Index }

func (uiListTool) Name() string { return "ui_list" }
func (uiListTool) Description() string {
	return "List indexed UI components (name + relative path). Optional limit."
}
func (uiListTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer"}}}`)
}
func (uiListTool) ReadOnly() bool { return true }
func (t uiListTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Limit int `json:"limit"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &p)
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 80
	}
	comps := t.idx.Components
	if len(comps) > limit {
		comps = comps[:limit]
	}
	b, _ := json.Marshal(comps)
	if len(comps) == 0 {
		return "No UI components indexed.\n[]", nil
	}
	return fmt.Sprintf("%d UI component(s):\n%s", len(comps), string(b)), nil
}

type uiFindTool struct{ idx *Index }

func (uiFindTool) Name() string { return "ui_find" }
func (uiFindTool) Description() string {
	return "Search UI components by export name or file path substring."
}
func (uiFindTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"}},"required":["query"]}`)
}
func (uiFindTool) ReadOnly() bool { return true }
func (t uiFindTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	comps := t.idx.Find(p.Query, p.Limit)
	b, _ := json.Marshal(comps)
	if len(comps) == 0 {
		return fmt.Sprintf("No UI components matched %q.\n[]", p.Query), nil
	}
	return fmt.Sprintf("%d match(es) for %q:\n%s", len(comps), p.Query, string(b)), nil
}

type uiReadTool struct{ idx *Index }

func (uiReadTool) Name() string { return "ui_read" }
func (uiReadTool) Description() string {
	return "Read a UI component file by export name or relative path."
}
func (uiReadTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"name_or_path":{"type":"string"},"offset":{"type":"integer"},"limit":{"type":"integer"}},"required":["name_or_path"]}`)
}
func (uiReadTool) ReadOnly() bool { return true }
func (t uiReadTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		NameOrPath string `json:"name_or_path"`
		Offset     int    `json:"offset"`
		Limit      int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	c, ok := t.idx.Lookup(p.NameOrPath)
	if !ok {
		return "", fmt.Errorf("ui component %q not found — try ui_find", p.NameOrPath)
	}
	b, err := os.ReadFile(c.Path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(b), "\n")
	offset := p.Offset
	if offset < 0 {
		offset = 0
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 120
	}
	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}
	if offset >= len(lines) {
		return fmt.Sprintf("%s (%s): offset %d beyond file (%d lines)", c.Name, c.Rel, offset, len(lines)), nil
	}
	var body strings.Builder
	for i := offset; i < end; i++ {
		fmt.Fprintf(&body, "%4d|%s\n", i+1, lines[i])
	}
	return fmt.Sprintf("%s (%s)\n%s", c.Name, c.Rel, body.String()), nil
}
