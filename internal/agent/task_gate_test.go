package agent

import (
	"context"
	"encoding/json"
	"testing"

	"arcdesk/internal/event"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
)

type labeledGate struct {
	label       string
	hasApprover bool
	checks      int
}

func (g *labeledGate) HasApprover() bool { return g.hasApprover }

func (g *labeledGate) Check(ctx context.Context, toolName string, args json.RawMessage, readOnly bool) (bool, string, error) {
	g.checks++
	return true, "", nil
}

func TestEffectiveSubagentGateUsesContextStamp(t *testing.T) {
	parent := &labeledGate{label: "parent", hasApprover: true}
	fallback := &labeledGate{label: "fallback", hasApprover: false}
	ctx := withCallContext(context.Background(), "c1", event.Discard, nil, parent)

	if got := EffectiveSubagentGate(ctx, fallback); got != parent {
		t.Fatal("want parent gate from context")
	}
	if fallback.checks != 0 {
		t.Fatalf("fallback consulted %d times, want 0", fallback.checks)
	}
}

func TestEffectiveSubagentGateFallsBackWithoutStamp(t *testing.T) {
	fallback := &labeledGate{label: "fallback", hasApprover: false}
	if got := EffectiveSubagentGate(context.Background(), fallback); got != fallback {
		t.Fatal("want fallback gate")
	}
}

type gateProbeTool struct {
	t        *testing.T
	wantGate Gate
	fallback Gate
}

func (p gateProbeTool) Name() string { return "probe" }
func (gateProbeTool) Description() string {
	return "probe"
}
func (gateProbeTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (gateProbeTool) ReadOnly() bool          { return true }
func (p gateProbeTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	got := EffectiveSubagentGate(ctx, p.fallback)
	if got != p.wantGate {
		p.t.Fatalf("EffectiveSubagentGate = %v, want %v", got, p.wantGate)
	}
	return "ok", nil
}

func TestExecuteOneStampsSubagentGateWhenDesktopInherit(t *testing.T) {
	parentGate := &labeledGate{label: "parent", hasApprover: true}
	reg := tool.NewRegistry()
	reg.Add(gateProbeTool{t: t, wantGate: parentGate})

	a := New(nil, reg, NewSession(""), Options{Gate: parentGate}, event.Discard)
	a.SetInheritSubagentGate(true)

	out := a.executeOne(context.Background(), provider.ToolCall{Name: "probe", Arguments: `{}`})
	if out.errMsg != "" {
		t.Fatalf("executeOne: %s", out.errMsg)
	}
}

func TestExecuteOneOmitsSubagentGateWithoutDesktopInherit(t *testing.T) {
	parentGate := &labeledGate{label: "parent", hasApprover: true}
	fallback := &labeledGate{label: "fallback", hasApprover: false}
	reg := tool.NewRegistry()
	reg.Add(gateProbeTool{t: t, wantGate: fallback, fallback: fallback})

	a := New(nil, reg, NewSession(""), Options{Gate: parentGate}, event.Discard)
	out := a.executeOne(context.Background(), provider.ToolCall{Name: "probe", Arguments: `{}`})
	if out.errMsg != "" {
		t.Fatalf("executeOne: %s", out.errMsg)
	}
}

type stepMockProvider struct {
	steps [][]provider.Chunk
	n     int
}

func (m *stepMockProvider) Name() string { return "step" }

func (m *stepMockProvider) Stream(_ context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	idx := m.n
	if idx >= len(m.steps) {
		idx = len(m.steps) - 1
	}
	m.n++
	chunks := m.steps[idx]
	ch := make(chan provider.Chunk, len(chunks))
	for _, c := range chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

func TestTaskSubagentInheritsDesktopInteractiveGate(t *testing.T) {
	parentGate := &labeledGate{label: "parent", hasApprover: true}
	headlessGate := &labeledGate{label: "headless", hasApprover: false}

	sub := &stepMockProvider{steps: [][]provider.Chunk{{
		{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "1", Name: "bash", Arguments: `{"command":"echo hi"}`}},
		{Type: provider.ChunkDone},
	}, {
		{Type: provider.ChunkText, Text: "done"},
		{Type: provider.ChunkDone},
	}}}
	parentReg := tool.NewRegistry()
	parentReg.Add(fakeTool{name: "bash", readOnly: false})
	taskTool := NewTaskTool(sub, nil, parentReg, 20, 0, 0, 0, 0, 0.0, "", "sys", headlessGate)
	parentReg.Add(taskTool)

	parent := New(nil, parentReg, NewSession(""), Options{Gate: parentGate}, event.Discard)
	parent.SetInheritSubagentGate(true)

	out := parent.executeOne(context.Background(), provider.ToolCall{
		Name: "task", Arguments: `{"prompt":"run echo"}`,
	})
	if out.errMsg != "" {
		t.Fatalf("task execute: %s (%s)", out.errMsg, out.output)
	}
	if headlessGate.checks != 0 {
		t.Fatalf("headless gate checks = %d, want 0", headlessGate.checks)
	}
	if parentGate.checks < 2 {
		t.Fatalf("parent gate checks = %d, want at least 2 (task + subagent bash)", parentGate.checks)
	}
}

func TestTaskSubagentKeepsHeadlessWithoutDesktopInherit(t *testing.T) {
	parentGate := &labeledGate{label: "parent", hasApprover: true}
	headlessGate := &labeledGate{label: "headless", hasApprover: false}

	sub := &stepMockProvider{steps: [][]provider.Chunk{{
		{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "1", Name: "bash", Arguments: `{"command":"echo hi"}`}},
		{Type: provider.ChunkDone},
	}, {
		{Type: provider.ChunkText, Text: "done"},
		{Type: provider.ChunkDone},
	}}}
	parentReg := tool.NewRegistry()
	parentReg.Add(fakeTool{name: "bash", readOnly: false})
	taskTool := NewTaskTool(sub, nil, parentReg, 20, 0, 0, 0, 0, 0.0, "", "sys", headlessGate)
	parentReg.Add(taskTool)

	parent := New(nil, parentReg, NewSession(""), Options{Gate: parentGate}, event.Discard)

	out := parent.executeOne(context.Background(), provider.ToolCall{
		Name: "task", Arguments: `{"prompt":"run echo"}`,
	})
	if out.errMsg != "" {
		t.Fatalf("task execute: %s (%s)", out.errMsg, out.output)
	}
	if headlessGate.checks != 1 {
		t.Fatalf("headless gate checks = %d, want 1 (subagent bash)", headlessGate.checks)
	}
	if parentGate.checks != 1 {
		t.Fatalf("parent gate checks = %d, want 1 (task dispatch only)", parentGate.checks)
	}
}
