package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds phased planner tools when tracker is configured.
func RegisterTools(reg *tool.Registry, tracker *Tracker) {
	if reg == nil || tracker == nil {
		return
	}
	reg.Add(plannerStatusTool{tracker: tracker})
	reg.Add(plannerLoadTool{tracker: tracker})
	reg.Add(plannerCompleteTool{tracker: tracker})
}

type plannerStatusTool struct{ tracker *Tracker }

func (plannerStatusTool) Name() string { return "planner_status" }
func (plannerStatusTool) Description() string {
	return "Report phased plan progress: total phases, current phase title, and completed summaries."
}
func (plannerStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (plannerStatusTool) ReadOnly() bool { return true }
func (t plannerStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	st := t.tracker.Status()
	b, _ := json.Marshal(st)
	if st.TotalPhases == 0 {
		return "No phased plan loaded.\n" + string(b), nil
	}
	var lines []string
	if st.Active {
		lines = append(lines, fmt.Sprintf("Phase %d/%d active: %s", st.CurrentIndex+1, st.TotalPhases, st.CurrentTitle))
	} else if st.CurrentIndex >= st.TotalPhases {
		lines = append(lines, fmt.Sprintf("All %d phases complete.", st.TotalPhases))
	}
	if len(st.Summaries) > 0 {
		lines = append(lines, "Completed:")
		for i, s := range st.Summaries {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, s))
		}
	}
	if focus := t.tracker.CurrentFocusBlock(); focus != "" {
		lines = append(lines, focus)
	}
	return strings.Join(lines, "\n") + "\n" + string(b), nil
}

type plannerLoadTool struct{ tracker *Tracker }

func (plannerLoadTool) Name() string { return "planner_load_plan" }
func (plannerLoadTool) Description() string {
	return "Parse a markdown plan into ordered phases and start phased execution at phase 1. Use after drafting a plan or when resuming structured work."
}
func (plannerLoadTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"plan":{"type":"string","description":"Markdown plan with top-level list items as phases"}},"required":["plan"]}`)
}
func (plannerLoadTool) ReadOnly() bool { return false }
func (t plannerLoadTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Plan string `json:"plan"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	t.tracker.LoadFromPlan(p.Plan)
	st := t.tracker.Status()
	if st.TotalPhases == 0 {
		return "No phases parsed — use top-level markdown list items (e.g. \"1. Step one\").", nil
	}
	focus := t.tracker.CurrentFocusBlock()
	return fmt.Sprintf("Loaded %d phase(s). %s\n%s", st.TotalPhases, st.CurrentTitle, focus), nil
}

type plannerCompleteTool struct{ tracker *Tracker }

func (plannerCompleteTool) Name() string { return "planner_complete_phase" }
func (plannerCompleteTool) Description() string {
	return "Mark the current phase complete and advance to the next. Call only after the current phase is implemented and verified."
}
func (plannerCompleteTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"summary":{"type":"string","description":"Brief note of what was done in this phase"}}}`)
}
func (plannerCompleteTool) ReadOnly() bool { return false }
func (t plannerCompleteTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var p struct {
		Summary string `json:"summary"`
	}
	if args != nil {
		_ = json.Unmarshal(args, &p)
	}
	next, err := t.tracker.CompleteCurrent(p.Summary)
	if err != nil {
		return "", err
	}
	return next, nil
}
