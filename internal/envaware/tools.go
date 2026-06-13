package envaware

import (
	"context"
	"encoding/json"
	"fmt"

	"arcdesk/internal/tool"
)

// RegisterTools adds environment_status for the probed snapshot.
func RegisterTools(reg *tool.Registry, snap Snapshot) {
	if reg == nil {
		return
	}
	reg.Add(envStatusTool{snap: snap})
}

type envStatusTool struct{ snap Snapshot }

func (envStatusTool) Name() string { return "environment_status" }
func (envStatusTool) Description() string {
	return "Report OS, shell, and installed Go/Node/npm/pnpm/git/gh/Wails versions plus platform notes."
}
func (envStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (envStatusTool) ReadOnly() bool { return true }
func (t envStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	b, err := json.MarshalIndent(t.snap, "", "  ")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Host environment:\n%s", string(b)), nil
}

// RefreshTool re-probes on each call (for long sessions).
type RefreshTool struct {
	Workspace string
}

func (RefreshTool) Name() string { return "environment_refresh" }
func (RefreshTool) Description() string {
	return "Re-probe the host toolchain and OS versions (use after installing dependencies mid-session)."
}
func (RefreshTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (RefreshTool) ReadOnly() bool { return true }
func (t RefreshTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	snap := Probe(ctx, t.Workspace)
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Refreshed host environment:\n%s", string(b)), nil
}

// RegisterRefreshTool adds environment_refresh when workspace is known.
func RegisterRefreshTool(reg *tool.Registry, workspace string) {
	if reg == nil || workspace == "" {
		return
	}
	reg.Add(RefreshTool{Workspace: workspace})
}
