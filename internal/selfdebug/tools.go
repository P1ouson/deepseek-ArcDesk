package selfdebug

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds selfdebug_status when a tracker is configured.
func RegisterTools(reg *tool.Registry, tracker *Tracker) {
	if reg == nil || tracker == nil {
		return
	}
	reg.Add(sdStatusTool{tracker: tracker})
}

type sdStatusTool struct{ tracker *Tracker }

func (sdStatusTool) Name() string { return "selfdebug_status" }
func (sdStatusTool) Description() string {
	return "Report the current self-debug loop phase: failed check, pending verifications, changed paths, and retry attempt."
}
func (sdStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (sdStatusTool) ReadOnly() bool { return true }
func (t sdStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	snap := t.tracker.Snapshot()
	plan := t.tracker.Plan()
	b, _ := json.Marshal(snap)
	var lines []string
	lines = append(lines, fmt.Sprintf("Self-debug loop: phase=%s attempt=%d/%d failed=%q",
		snap.Phase, snap.Attempt, snap.MaxRetries, snap.FailedCmd))
	if len(snap.PendingChecks) > 0 {
		lines = append(lines, "Pending checks:")
		for _, c := range snap.PendingChecks {
			lines = append(lines, "- "+c)
		}
	}
	if len(plan.Checks) > 0 {
		lines = append(lines, fmt.Sprintf("Configured checks: %d (max_retries=%d on_failure=%s)",
			len(plan.Checks), plan.Policy.MaxRetries, plan.Policy.OnFailure))
	}
	return strings.Join(lines, "\n") + "\n" + string(b), nil
}
