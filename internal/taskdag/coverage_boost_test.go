package taskdag

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestTrackerLifecycle(t *testing.T) {
	tr := NewTracker()
	tr.LoadFromPlan("")
	if tr.Status().Total != 0 {
		t.Fatal("empty plan should clear")
	}
	tr.LoadFromPlan(`[{"id":"a","title":"A"},{"id":"b","title":"B","deps":["a"]}]`)
	st := tr.Status()
	if st.Total != 2 || len(st.ReadyIDs) != 1 || st.ReadyIDs[0] != "a" {
		t.Fatalf("status=%+v", st)
	}
	if err := tr.Start("b"); err == nil {
		t.Fatal("blocked task should not start")
	}
	if err := tr.Start("nope"); err == nil {
		t.Fatal("unknown start")
	}
	if _, err := tr.Complete("nope", ""); err == nil {
		t.Fatal("unknown complete")
	}
	if _, err := tr.Complete("b", ""); err == nil {
		t.Fatal("pending complete")
	}
	if err := tr.Start("a"); err != nil {
		t.Fatal(err)
	}
	msg, err := tr.Complete("a", "done")
	if err != nil || !strings.Contains(msg, "Ready now: b") {
		t.Fatalf("msg=%q err=%v", msg, err)
	}
	if err := tr.Start("b"); err != nil {
		t.Fatal(err)
	}
	msg, err = tr.Complete("b", "")
	if err != nil || !strings.Contains(msg, "All tasks complete") {
		t.Fatalf("final=%q err=%v", msg, err)
	}
	tr.Clear()
	if tr.Status().Total != 0 {
		t.Fatal("cleared")
	}
}

func TestLoadFromPlanJSONObject(t *testing.T) {
	tr := NewTracker()
	tr.LoadFromPlan(`{"nodes":[{"id":"x","title":"X"}]}`)
	if len(tr.Ready()) != 1 {
		t.Fatalf("ready=%+v", tr.Ready())
	}
}

func TestValidateCycle(t *testing.T) {
	tr := NewTracker()
	tr.Load([]Node{
		{ID: "a", Title: "A", Deps: []string{"b"}},
		{ID: "b", Title: "B", Deps: []string{"a"}},
	})
	issues := tr.ValidateIssues()
	if len(issues) == 0 || !strings.Contains(issues[0], "cycle") {
		t.Fatalf("issues=%v", issues)
	}
	if len(tr.Ready()) != 0 {
		t.Fatal("cycle should block ready tasks")
	}
}

func TestNilTrackerSafe(t *testing.T) {
	var tr *Tracker
	tr.LoadFromPlan("1. x")
	tr.Clear()
	if tr.Status().Total != 0 {
		t.Fatal("nil status")
	}
	if tr.Ready() != nil {
		t.Fatal("nil ready")
	}
	if _, err := tr.Complete("a", ""); err == nil {
		t.Fatal("nil complete")
	}
	if err := tr.Start("a"); err == nil {
		t.Fatal("nil start")
	}
	if tr.ValidateIssues() != nil {
		t.Fatal("nil validate")
	}
}

func TestTaskDAGToolsExecute(t *testing.T) {
	tr := NewTracker()
	reg := tool.NewRegistry()
	RegisterTools(reg, tr)
	for _, name := range []string{"taskdag_status", "taskdag_load", "taskdag_ready", "taskdag_start", "taskdag_complete"} {
		tl, ok := reg.Get(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if tl.Name() == "" || tl.Description() == "" {
			t.Fatalf("meta for %s", name)
		}
	}
	status, _ := reg.Get("taskdag_status")
	out, err := status.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "No task DAG") {
		t.Fatalf("status=%q err=%v", out, err)
	}
	load, _ := reg.Get("taskdag_load")
	out, err = load.Execute(context.Background(), json.RawMessage(`{"plan":"- a: Alpha\n- b: Beta (deps: a)"}`))
	if err != nil || !strings.Contains(out, "Loaded 2 task(s)") {
		t.Fatalf("load=%q err=%v", out, err)
	}
	out, err = load.Execute(context.Background(), json.RawMessage(`{"plan":"- x: X (deps: missing)"}`))
	if err != nil || !strings.Contains(out, "Warnings") {
		t.Fatalf("load warn=%q err=%v", out, err)
	}
	out, err = load.Execute(context.Background(), json.RawMessage(`{"plan":"- a: Alpha\n- b: Beta (deps: a)"}`))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	out, err = load.Execute(context.Background(), json.RawMessage(`{"plan":"\n\n"}`))
	if err != nil || !strings.Contains(out, "No tasks parsed") {
		t.Fatalf("load empty=%q err=%v", out, err)
	}
	out, err = load.Execute(context.Background(), json.RawMessage(`{"plan":"- a: Alpha\n- b: Beta (deps: a)"}`))
	if err != nil {
		t.Fatalf("reload before start: %v", err)
	}
	if _, err = load.Execute(context.Background(), json.RawMessage(`{`)); err == nil {
		t.Fatal("invalid load args")
	}
	ready, _ := reg.Get("taskdag_ready")
	out, err = ready.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "ready task") {
		t.Fatalf("ready=%q err=%v", out, err)
	}
	start, _ := reg.Get("taskdag_start")
	if _, err = start.Execute(context.Background(), json.RawMessage(`{"id":"a"}`)); err != nil {
		t.Fatalf("start: %v", err)
	}
	complete, _ := reg.Get("taskdag_complete")
	out, err = complete.Execute(context.Background(), json.RawMessage(`{"id":"a","summary":"ok"}`))
	if err != nil || !strings.Contains(out, "Completed") {
		t.Fatalf("complete=%q err=%v", out, err)
	}
	if _, err = complete.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("complete needs id")
	}
}

func TestTaskDAGRegisterNilSafe(t *testing.T) {
	RegisterTools(nil, NewTracker())
	RegisterTools(tool.NewRegistry(), nil)
}

func TestTaskDAGStatusAndReadyEmpty(t *testing.T) {
	tr := NewTracker()
	tr.Load([]Node{{ID: "a", Title: "A"}})
	st := tr.Status()
	if st.Active != true || len(st.ReadyIDs) != 1 {
		t.Fatalf("status=%+v", st)
	}
	reg := tool.NewRegistry()
	RegisterTools(reg, tr)
	status, _ := reg.Get("taskdag_status")
	out, err := status.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "0/1") {
		t.Fatalf("status=%q err=%v", out, err)
	}
	readyTool, _ := reg.Get("taskdag_ready")
	out, err = readyTool.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, `"id":"a"`) {
		t.Fatalf("ready=%q err=%v", out, err)
	}
}

func TestLoadDuplicateID(t *testing.T) {
	tr := NewTracker()
	tr.Load([]Node{{ID: "a", Title: "One"}, {ID: "a", Title: "Two"}})
	if len(tr.order) != 2 {
		t.Fatalf("order=%v", tr.order)
	}
	if tr.nodes["a"].Title != "One" || tr.nodes["a-2"].Title != "Two" {
		t.Fatalf("nodes=%+v %+v", tr.nodes["a"], tr.nodes["a-2"])
	}
}

func TestCompleteDirectlyFromReady(t *testing.T) {
	tr := NewTracker()
	tr.Load([]Node{{ID: "a", Title: "A"}})
	msg, err := tr.Complete("a", "skipped start")
	if err != nil || !strings.Contains(msg, "All tasks complete") {
		t.Fatalf("msg=%q err=%v", msg, err)
	}
}

func TestLoadFromPlanClearsOnEmpty(t *testing.T) {
	tr := NewTracker()
	tr.Load([]Node{{ID: "x", Title: "X"}})
	tr.LoadFromPlan("")
	if tr.Status().Total != 0 {
		t.Fatalf("empty plan should clear, total=%d", tr.Status().Total)
	}
}

func TestUniqueIDCollision(t *testing.T) {
	used := map[string]bool{}
	id1 := uniqueID("task", used)
	id2 := uniqueID("task", used)
	if id1 == id2 {
		t.Fatalf("ids collided: %q %q", id1, id2)
	}
}

func TestTaskDAGReadyEmptyAfterLoad(t *testing.T) {
	tr := NewTracker()
	reg := tool.NewRegistry()
	RegisterTools(reg, tr)
	readyTool, _ := reg.Get("taskdag_ready")
	out, err := readyTool.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "No ready tasks") {
		t.Fatalf("ready empty=%q err=%v", out, err)
	}
}

func TestTaskDAGStatusRunningAndBlocked(t *testing.T) {
	tr := NewTracker()
	tr.Load([]Node{{ID: "a", Title: "A"}, {ID: "b", Title: "B", Deps: []string{"a"}}})
	_ = tr.Start("a")
	st := tr.Status()
	if len(st.RunningIDs) != 1 || st.BlockedCount != 1 {
		t.Fatalf("status=%+v", st)
	}
}

func TestSlugIDNonAlpha(t *testing.T) {
	if slugID("***") != "task" {
		t.Fatal("non alpha slug")
	}
}
