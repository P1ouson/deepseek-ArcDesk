package taskdag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds taskdag_* orchestration tools.
func RegisterTools(reg *tool.Registry, tracker *Tracker) {
	if reg == nil || tracker == nil {
		return
	}
	reg.Add(taskdagStatusTool{tracker: tracker})
	reg.Add(taskdagLoadTool{tracker: tracker})
	reg.Add(taskdagReadyTool{tracker: tracker})
	reg.Add(taskdagStartTool{tracker: tracker})
	reg.Add(taskdagCompleteTool{tracker: tracker})
}

type taskdagStatusTool struct{ tracker *Tracker }

func (taskdagStatusTool) Name() string { return "taskdag_status" }
func (taskdagStatusTool) Description() string {
	return "Report task DAG progress: totals, ready tasks, and running tasks."
}
func (taskdagStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (taskdagStatusTool) ReadOnly() bool { return true }
func (t taskdagStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	st := t.tracker.Status()
	b, _ := json.Marshal(st)
	if st.Total == 0 {
		return "No task DAG loaded.\n" + string(b), nil
	}
	return fmt.Sprintf("Task DAG: %d/%d done, %d ready, %d running\n%s",
		st.DoneCount, st.Total, len(st.ReadyIDs), len(st.RunningIDs), string(b)), nil
}

type taskdagLoadTool struct{ tracker *Tracker }

func (taskdagLoadTool) Name() string { return "taskdag_load" }
func (taskdagLoadTool) Description() string {
	return "Load a dependency-ordered task graph from markdown list items or JSON {\"nodes\":[...]}."
}
func (taskdagLoadTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"plan":{"type":"string"}},"required":["plan"]}`)
}
func (taskdagLoadTool) ReadOnly() bool { return false }
func (t taskdagLoadTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Plan string `json:"plan"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	t.tracker.LoadFromPlan(p.Plan)
	st := t.tracker.Status()
	if st.Total == 0 {
		return "No tasks parsed — use list items like \"- auth: Add login (deps: db)\" or JSON nodes.", nil
	}
	ready := t.tracker.Ready()
	var names []string
	for _, n := range ready {
		names = append(names, n.ID)
	}
	msg := fmt.Sprintf("Loaded %d task(s). Ready: %s", st.Total, strings.Join(names, ", "))
	if issues := t.tracker.ValidateIssues(); len(issues) > 0 {
		msg += "\nWarnings:\n- " + strings.Join(issues, "\n- ")
	}
	return msg, nil
}

type taskdagReadyTool struct{ tracker *Tracker }

func (taskdagReadyTool) Name() string { return "taskdag_ready" }
func (taskdagReadyTool) Description() string {
	return "List tasks whose dependencies are satisfied and can run in parallel."
}
func (taskdagReadyTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (taskdagReadyTool) ReadOnly() bool { return true }
func (t taskdagReadyTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	ready := t.tracker.Ready()
	b, _ := json.Marshal(ready)
	if len(ready) == 0 {
		return "No ready tasks.\n[]", nil
	}
	return fmt.Sprintf("%d ready task(s):\n%s", len(ready), string(b)), nil
}

type taskdagStartTool struct{ tracker *Tracker }

func (taskdagStartTool) Name() string { return "taskdag_start" }
func (taskdagStartTool) Description() string {
	return "Mark a ready task as running before delegating work (e.g. via task tool)."
}
func (taskdagStartTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`)
}
func (taskdagStartTool) ReadOnly() bool { return false }
func (t taskdagStartTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if err := t.tracker.Start(p.ID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Started task %q.", p.ID), nil
}

type taskdagCompleteTool struct{ tracker *Tracker }

func (taskdagCompleteTool) Name() string { return "taskdag_complete" }
func (taskdagCompleteTool) Description() string {
	return "Mark a running task complete and unlock dependents."
}
func (taskdagCompleteTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"},"summary":{"type":"string"}},"required":["id"]}`)
}
func (taskdagCompleteTool) ReadOnly() bool { return false }
func (t taskdagCompleteTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	return t.tracker.Complete(p.ID, p.Summary)
}
