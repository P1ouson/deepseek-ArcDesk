package ctxcompress

import (
	"context"
	"encoding/json"

	"arcdesk/internal/tool"
)

// RegisterTools adds context_compression_status.
func RegisterTools(reg *tool.Registry, c *Compressor) {
	if reg == nil || c == nil {
		return
	}
	reg.Add(ctxCompressStatusTool{c: c})
}

type ctxCompressStatusTool struct{ c *Compressor }

func (ctxCompressStatusTool) Name() string { return "context_compression_status" }
func (ctxCompressStatusTool) Description() string {
	return "Report tool-output compression settings for long context sessions."
}
func (ctxCompressStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (ctxCompressStatusTool) ReadOnly() bool { return true }
func (t ctxCompressStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	b, _ := json.Marshal(t.c.Snapshot())
	return "Context compression:\n" + string(b), nil
}
