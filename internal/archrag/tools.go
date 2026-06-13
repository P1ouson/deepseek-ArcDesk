package archrag

import (
	"context"
	"encoding/json"
	"fmt"

	"arcdesk/internal/tool"
)

// RegisterTools adds architecture document retrieval tools to reg.
func RegisterTools(reg *tool.Registry, idx *Index) {
	if reg == nil || idx == nil || len(idx.summary) == 0 {
		return
	}
	reg.Add(archListTool{idx: idx})
	reg.Add(archFindTool{idx: idx})
	reg.Add(archReadTool{idx: idx})
}

type archListTool struct{ idx *Index }

func (archListTool) Name() string { return "architecture_list" }
func (archListTool) Description() string {
	return "List indexed architecture/policy documents (SPEC, SECURITY, README, etc.) and their section headings."
}
func (archListTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (archListTool) ReadOnly() bool { return true }
func (t archListTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	docs := t.idx.List()
	b, _ := json.Marshal(docs)
	if len(docs) == 0 {
		return "No architecture documents indexed.\n[]", nil
	}
	return fmt.Sprintf("%d architecture document(s):\n%s", len(docs), string(b)), nil
}

type archFindTool struct{ idx *Index }

func (archFindTool) Name() string { return "architecture_find" }
func (archFindTool) Description() string {
	return "Search architecture document sections by keyword (heading or body). Use before large refactors to load relevant SPEC/SECURITY constraints."
}
func (archFindTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"doc":{"type":"string","description":"Optional repo-relative doc path to scope search"},"limit":{"type":"integer"}},"required":["query"]}`)
}
func (archFindTool) ReadOnly() bool { return true }
func (t archFindTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Query string `json:"query"`
		Doc   string `json:"doc"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	sections := t.idx.FindSections(p.Doc, p.Query, p.Limit)
	b, _ := json.Marshal(sections)
	if len(sections) == 0 {
		return fmt.Sprintf("No sections matched %q.\n[]", p.Query), nil
	}
	return fmt.Sprintf("%d section(s) matched %q:\n%s", len(sections), p.Query, string(b)), nil
}

type archReadTool struct{ idx *Index }

func (archReadTool) Name() string { return "architecture_read" }
func (archReadTool) Description() string {
	return "Read an indexed architecture document by repo-relative path (from architecture_list)."
}
func (archReadTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"doc":{"type":"string"}},"required":["doc"]}`)
}
func (archReadTool) ReadOnly() bool { return true }
func (t archReadTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Doc string `json:"doc"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	text, ok := t.idx.ReadDoc(p.Doc)
	if !ok {
		return "", fmt.Errorf("document %q is not indexed or missing on disk", p.Doc)
	}
	return text, nil
}
