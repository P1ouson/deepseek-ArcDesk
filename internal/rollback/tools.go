package rollback

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds rollback_status and rollback_diff when a host is configured.
func RegisterTools(reg *tool.Registry, host *Host) {
	if reg == nil || host == nil {
		return
	}
	reg.Add(rollbackStatusTool{host: host})
	reg.Add(rollbackDiffTool{host: host})
}

type rollbackStatusTool struct{ host *Host }

func (rollbackStatusTool) Name() string { return "rollback_status" }
func (rollbackStatusTool) Description() string {
	return "List session checkpoints and the active turn available for structured rollback diffs."
}
func (rollbackStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (rollbackStatusTool) ReadOnly() bool { return true }

func (t rollbackStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	cps := t.host.Checkpoints()
	active := t.host.ActiveTurn()
	var lines []string
	lines = append(lines, fmt.Sprintf("Checkpoints: %d active_turn=%d", len(cps), active))
	for _, cp := range cps {
		lines = append(lines, fmt.Sprintf("- turn %d: %q (%d file(s))", cp.Turn, truncate(cp.Prompt, 60), len(cp.Paths)))
	}
	return strings.Join(lines, "\n"), nil
}

type rollbackDiffTool struct{ host *Host }

func (rollbackDiffTool) Name() string { return "rollback_diff" }
func (rollbackDiffTool) Description() string {
	return "Build a structured auto-diff for rewinding a turn (current workspace → checkpoint state)."
}
func (rollbackDiffTool) Schema() json.RawMessage {
	return json.RawMessage(`{
  "type":"object",
  "properties":{
    "turn":{"type":"integer","description":"Checkpoint turn to preview; defaults to active turn"}
  }
}`)
}
func (rollbackDiffTool) ReadOnly() bool { return true }

func (t rollbackDiffTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var in struct {
		Turn *int `json:"turn"`
	}
	_ = json.Unmarshal(args, &in)
	turn := t.host.ActiveTurn()
	if in.Turn != nil {
		turn = *in.Turn
	}
	if turn < 0 {
		return "", fmt.Errorf("no active turn for rollback preview")
	}
	report := t.host.Report(turn)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Rollback diff for turn %d: %s\n", report.Turn, report.Summary))
	if block := FormatDiffBlock(report, 8); block != "" {
		b.WriteString(block)
		b.WriteByte('\n')
	}
	b.WriteString(FormatJSON(report))
	return b.String(), nil
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
