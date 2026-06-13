package planner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestParsePhasesVariants(t *testing.T) {
	plan := `1. Phase one
   - sub
2. two
1) third
**bold phase**
`
	phases := ParsePhases(plan)
	if len(phases) < 3 {
		t.Fatalf("phases=%v", phases)
	}
	if phases[0].Title != "Phase one" || len(phases[0].Steps) != 1 {
		t.Fatalf("first phase=%+v", phases[0])
	}
	if phases[2].Title != "third" {
		t.Fatalf("third=%+v", phases[2])
	}
	bold := ParsePhases("1. **bold phase**\n")
	if len(bold) != 1 || bold[0].Title != "bold phase" {
		t.Fatalf("bold=%+v", bold)
	}
	many := strings.Builder{}
	for i := 1; i <= 15; i++ {
		many.WriteString("1. phase ")
		many.WriteString(strings.Repeat("x", 2))
		many.WriteByte('\n')
	}
	if len(ParsePhases(many.String())) != 12 {
		t.Fatal("should cap at 12 phases")
	}
	if ParsePhases("") != nil {
		t.Fatal("empty plan")
	}
}

func TestTrackerNilAndLoadPhases(t *testing.T) {
	var tr *Tracker
	tr.LoadFromPlan("1. x")
	tr.Clear()
	if tr.Status().TotalPhases != 0 {
		t.Fatal("nil tracker status")
	}
	if _, err := tr.CompleteCurrent("x"); err == nil {
		t.Fatal("nil complete")
	}
	if tr.BlockFinalAnswer() != "" {
		t.Fatal("nil block")
	}
	tr = NewTracker(false)
	tr.LoadPhases([]Phase{{Title: "only", Steps: []string{"a"}}})
	if tr.BlockFinalAnswer() != "" {
		t.Fatal("gates off")
	}
	next, err := tr.CompleteCurrent("")
	if err != nil || !strings.Contains(next, "All phases complete") {
		t.Fatalf("complete empty summary: %q err=%v", next, err)
	}
}

func TestTrackerCompleteErrors(t *testing.T) {
	tr := NewTracker(true)
	if _, err := tr.CompleteCurrent("x"); err == nil {
		t.Fatal("no plan")
	}
	tr.LoadFromPlan("1. one")
	tr.CompleteCurrent("done")
	if _, err := tr.CompleteCurrent("again"); err == nil {
		t.Fatal("no active phase")
	}
}

func TestPlannerToolsEdgeCases(t *testing.T) {
	tr := NewTracker(true)
	reg := tool.NewRegistry()
	RegisterTools(reg, tr)
	ctx := context.Background()
	load, _ := reg.Get("planner_load_plan")
	out, err := load.Execute(ctx, json.RawMessage(`{"plan":"not a list"}`))
	if err != nil || !strings.Contains(out, "No phases parsed") {
		t.Fatalf("empty parse: %q err=%v", out, err)
	}
	if _, err := load.Execute(ctx, json.RawMessage(`{`)); err == nil {
		t.Fatal("bad load json")
	}
	status, _ := reg.Get("planner_status")
	out, err = status.Execute(ctx, json.RawMessage(`{}`))
	if err != nil || !strings.Contains(out, "No phased plan") {
		t.Fatalf("idle status: %q err=%v", out, err)
	}
	load.Execute(ctx, json.RawMessage(`{"plan":"1. alpha\n2. beta"}`))
	tr.CompleteCurrent("alpha done")
	tr.CompleteCurrent("beta done")
	out, err = status.Execute(ctx, json.RawMessage(`{}`))
	if err != nil || !strings.Contains(out, "All 2 phases complete") {
		t.Fatalf("done status: %q err=%v", out, err)
	}
	complete, _ := reg.Get("planner_complete_phase")
	if _, err := complete.Execute(ctx, json.RawMessage(`{}`)); err == nil {
		t.Fatal("complete with no active phase")
	}
}

func TestRegisterToolsNil(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterTools(nil, nil)
	RegisterTools(reg, nil)
	if reg.Len() != 0 {
		t.Fatal("no tools")
	}
}

func TestCurrentFocusBlock(t *testing.T) {
	tr := NewTracker(true)
	if tr.CurrentFocusBlock() != "" {
		t.Fatal("empty focus")
	}
	tr.LoadFromPlan("1. Build\n   - step a\n   - step b")
	focus := tr.CurrentFocusBlock()
	if !strings.Contains(focus, "PHASED EXECUTION") || !strings.Contains(focus, "step a") {
		t.Fatalf("focus=%q", focus)
	}
}
