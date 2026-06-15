package harness

import (
	"strings"
	"testing"
)

func TestFSMPlanExecuteFlow(t *testing.T) {
	f := NewFSM()
	if got := f.BeginTurn(true, false); got != PhasePlan {
		t.Fatalf("BeginTurn plan = %q", got)
	}
	if got := f.Apply(EventPlanRejected); got != PhaseReplan {
		t.Fatalf("reject = %q", got)
	}
	if got := f.Apply(EventPlanResearch); got != PhasePlan {
		t.Fatalf("replan research = %q", got)
	}
	if got := f.Apply(EventPlanApproved); got != PhaseExecute {
		t.Fatalf("approved = %q", got)
	}
	if got := f.Apply(EventVerifyActive); got != PhaseVerify {
		t.Fatalf("verify = %q", got)
	}
	if got := f.Apply(EventTurnComplete); got != PhaseIdle {
		t.Fatalf("done = %q", got)
	}
}

func TestFSMVerifyExhaustHalt(t *testing.T) {
	f := NewFSM()
	f.BeginTurn(false, false)
	if got := f.Apply(EventVerifyExhaust); got != PhaseHalt {
		t.Fatalf("exhaust = %q", got)
	}
}

func TestFourLayerWorkingDrain(t *testing.T) {
	m := NewFourLayer()
	m.QueueWorking("a")
	m.QueueWorking("b")
	got := m.DrainWorking()
	if len(got) != 2 {
		t.Fatalf("drain len = %d", len(got))
	}
	if m.WorkingPending() != 0 {
		t.Fatal("expected empty after drain")
	}
	block := FormatWorkingBlock(got)
	if block == "" || !strings.Contains(block, "a") {
		t.Fatalf("block = %q", block)
	}
}

func TestDescribeLayers(t *testing.T) {
	rows := DescribeLayers(LayerFlags{ControlPlane: true, Skills: 2, MCP: 1})
	if len(rows) != len(AllLayers) {
		t.Fatalf("rows = %d", len(rows))
	}
}

func TestStatusUsesLiveLayers(t *testing.T) {
	p := NewPlane(Config{Layers: LayerFlags{MCP: 0, Skills: 0}})
	p.BindLiveLayers(LiveLayers{
		MCPServerCount: func() int { return 2 },
		SkillCount:     func() int { return 3 },
		SemanticLoaded: func() bool { return true },
	})
	st := p.Status()
	var mcpActive, skillsActive bool
	for _, row := range st.Layers {
		switch row.Layer {
		case LayerMCP:
			mcpActive = row.Active
		case LayerSkills:
			skillsActive = row.Active
		}
	}
	if !mcpActive {
		t.Fatal("expected MCP layer active from live count")
	}
	if !skillsActive {
		t.Fatal("expected skills layer active from live count")
	}
	if loaded, _ := st.Memory["semantic_loaded"].(bool); !loaded {
		t.Fatalf("semantic_loaded = %v, want true", st.Memory["semantic_loaded"])
	}
}
