package costrouter

import (
	"context"
	"encoding/json"
	"fmt"

	"arcdesk/internal/tool"
)

// RegisterTools adds cost_router_* tools.
func RegisterTools(reg *tool.Registry, router *Router) {
	if reg == nil || router == nil {
		return
	}
	reg.Add(costRouterStatusTool{router: router})
	reg.Add(costRouterClassifyTool{router: router})
}

type costRouterStatusTool struct{ router *Router }

func (costRouterStatusTool) Name() string { return "cost_router_status" }
func (costRouterStatusTool) Description() string {
	return "Report task-tier model routing configuration (classify/execute/compact/explore)."
}
func (costRouterStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (costRouterStatusTool) ReadOnly() bool { return true }
func (t costRouterStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	b, _ := json.Marshal(t.router.Snapshot())
	return "Cost router:\n" + string(b), nil
}

type costRouterClassifyTool struct{ router *Router }

func (costRouterClassifyTool) Name() string { return "cost_router_classify" }
func (costRouterClassifyTool) Description() string {
	return "Classify a prompt into a routing tier and show which model would be selected."
}
func (costRouterClassifyTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"prompt":{"type":"string"},"default_model":{"type":"string"}},"required":["prompt"]}`)
}
func (costRouterClassifyTool) ReadOnly() bool { return true }
func (t costRouterClassifyTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Prompt       string `json:"prompt"`
		DefaultModel string `json:"default_model"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	tier, model := t.router.ResolveModel(p.Prompt, p.DefaultModel)
	out := map[string]string{"tier": string(tier), "model": model}
	b, _ := json.Marshal(out)
	return string(b), nil
}
