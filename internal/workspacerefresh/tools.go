package workspacerefresh

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds workspace_refresh_status (read-only refresh plan + last run).
func RegisterTools(reg *tool.Registry, host *Host) {
	if reg == nil || host == nil {
		return
	}
	reg.Add(refreshStatusTool{host: host})
}

type refreshStatusTool struct{ host *Host }

func (refreshStatusTool) Name() string { return "workspace_refresh_status" }

func (refreshStatusTool) Description() string {
	return "Repo index refresh plan and last orchestrated run: repomap, dependency, callgraph, codegraph. Use before repeated dependency_status/codegraph probes — shows skip vs full rebuild vs meta-only bump."
}

func (refreshStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}

func (refreshStatusTool) ReadOnly() bool { return true }

func (t refreshStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	if t.host == nil {
		return "workspace refresh not configured", nil
	}
	plan := BuildPlan(t.host.PlanInput())
	last := t.host.LastReport()
	out := struct {
		Plan       Plan   `json:"plan"`
		LastRun    Report `json:"lastRun,omitempty"`
		Summary    string `json:"summary"`
		ReuseHint  string `json:"reuseHint,omitempty"`
	}{
		Plan:    plan,
		LastRun: last,
		Summary: formatSummary(plan),
	}
	if plan.SkipCount > 0 && plan.RefreshCount == 0 {
		out.ReuseHint = "indexes fresh — prefer cached reads over re-scanning the tree"
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func formatSummary(plan Plan) string {
	var parts []string
	for _, layer := range plan.Layers {
		parts = append(parts, fmt.Sprintf("%s=%s(%s)", layer.Name, layer.Action, layer.Reason))
	}
	head := shortHead(plan.GitHead)
	if head != "" {
		return "head=" + head + " " + strings.Join(parts, " ")
	}
	return strings.Join(parts, " ")
}
