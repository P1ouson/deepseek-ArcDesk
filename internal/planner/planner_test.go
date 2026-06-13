package planner

import (
	"context"
	"encoding/json"
	"testing"

	"arcdesk/internal/tool"
)

func TestParsePhases(t *testing.T) {
	plan := `## Plan
1. Add loader
   - wire boot
   - add tests
2. Update docs`
	phases := ParsePhases(plan)
	if len(phases) != 2 {
		t.Fatalf("phases=%d", len(phases))
	}
	if phases[0].Title != "Add loader" || len(phases[0].Steps) != 2 {
		t.Fatalf("phase0=%+v", phases[0])
	}
	if phases[1].Title != "Update docs" {
		t.Fatalf("phase1=%+v", phases[1])
	}
}

func TestTrackerGates(t *testing.T) {
	tr := NewTracker(true)
	tr.LoadFromPlan("1. first\n2. second")
	if got := tr.BlockFinalAnswer(); got == "" {
		t.Fatal("expected gate while phase active")
	}
	if _, err := tr.CompleteCurrent("done first"); err != nil {
		t.Fatal(err)
	}
	if got := tr.BlockFinalAnswer(); got == "" {
		t.Fatal("expected gate on phase 2")
	}
	if _, err := tr.CompleteCurrent("done second"); err != nil {
		t.Fatal(err)
	}
	if got := tr.BlockFinalAnswer(); got != "" {
		t.Fatalf("unexpected gate: %q", got)
	}
}

func TestTrackerClearDisarmsGate(t *testing.T) {
	tr := NewTracker(true)
	tr.LoadFromPlan("1. only")
	if tr.BlockFinalAnswer() == "" {
		t.Fatal("expected armed gate")
	}
	tr.Clear()
	if got := tr.BlockFinalAnswer(); got != "" {
		t.Fatalf("clear should disarm gate, got %q", got)
	}
}

func TestPlannerTools(t *testing.T) {
	tr := NewTracker(true)
	reg := tool.NewRegistry()
	RegisterTools(reg, tr)
	ctx := context.Background()
	loadTool, ok := reg.Get("planner_load_plan")
	if !ok {
		t.Fatal("missing planner_load_plan")
	}
	loadArgs, _ := json.Marshal(map[string]string{"plan": "1. alpha\n2. beta"})
	if _, err := loadTool.Execute(ctx, loadArgs); err != nil {
		t.Fatal(err)
	}
	statusTool, _ := reg.Get("planner_status")
	if out, err := statusTool.Execute(ctx, json.RawMessage(`{}`)); err != nil || out == "" {
		t.Fatalf("status: %q err=%v", out, err)
	}
	completeTool, _ := reg.Get("planner_complete_phase")
	completeArgs, _ := json.Marshal(map[string]string{"summary": "built alpha"})
	if out, err := completeTool.Execute(ctx, completeArgs); err != nil {
		t.Fatal(err)
	} else if out == "" {
		t.Fatal("expected next focus")
	}
}
