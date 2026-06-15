package harness

import (
	"context"
	"encoding/json"

	"arcdesk/internal/tool"
)

// RegisterTools adds harness introspection tools to reg.
func RegisterTools(reg *tool.Registry, plane *Plane) {
	if reg == nil || plane == nil {
		return
	}
	reg.Add(statusTool{plane: plane})
}

type statusTool struct {
	plane *Plane
}

func (statusTool) Name() string { return "harness_status" }

func (statusTool) Description() string {
	return "Return the Harness control plane snapshot: seven layers, current PLAN-EXECUTE-VERIFY phase, and four-layer memory status. Call this before searching the codebase for orchestration, memory, or RAG entry points."
}

func (statusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}

func (statusTool) ReadOnly() bool { return true }

func (t statusTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	b, err := json.MarshalIndent(t.plane.Status(), "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
