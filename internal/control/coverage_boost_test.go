package control

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"arcdesk/internal/agent"
	"arcdesk/internal/command"
	"arcdesk/internal/event"
	"arcdesk/internal/hook"
	"arcdesk/internal/memory"
	"arcdesk/internal/provider"
	"arcdesk/internal/skill"
	"arcdesk/internal/tool"
)

type ctrlRecordSink struct {
	mu     sync.Mutex
	texts  []string
	events []event.Event
}

func (s *ctrlRecordSink) Emit(e event.Event) {
	s.mu.Lock()
	s.texts = append(s.texts, e.Text)
	s.events = append(s.events, e)
	s.mu.Unlock()
}

func (s *ctrlRecordSink) joined() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.texts, "\n")
}

func testController(t *testing.T) *Controller {
	t.Helper()
	sess := agent.NewSession("system prompt")
	exec := agent.New(nil, tool.NewRegistry(), sess, agent.Options{
		ContextWindow: 8000,
		CompactRatio:  0.8,
	}, event.Discard)
	mem := &memory.Set{
		Docs: []memory.Source{{Scope: "project", Path: "ARCDESK.md"}},
	}
	skills := []skill.Skill{{Name: "explore", Description: "explore repo", RunAs: skill.RunSubagent}}
	sink := &ctrlRecordSink{}
	c := New(Options{
		Runner:        exec,
		Executor:      exec,
		Sink:          sink,
		Label:         "test",
		SystemPrompt:  "system prompt",
		SessionDir:    t.TempDir(),
		Skills:        skills,
		AllSkills:     skills,
		Memory:        mem,
		Hooks:         hook.NewRunner(nil, "", nil, nil),
		Registry:      tool.NewRegistry(),
		WorkspaceRoot: t.TempDir(),
		Commands:      []command.Command{{Name: "demo", Description: "demo cmd"}},
	})
	c.SetSessionPath(filepath.Join(t.TempDir(), "session.jsonl"))
	return c
}

func TestControllerGetterAPIs(t *testing.T) {
	c := testController(t)
	if len(c.Commands()) == 0 {
		t.Fatal("commands empty")
	}
	if len(c.Skills()) == 0 || len(c.AllSkills()) == 0 {
		t.Fatal("skills empty")
	}
	if c.Host() != nil {
		t.Fatal("expected nil host")
	}
	if c.HookRunner() == nil {
		t.Fatal("hook runner nil")
	}
	if c.Memory() == nil || len(c.Memory().Docs) == 0 {
		t.Fatal("memory missing")
	}
	if c.SessionDir() == "" || c.SessionPath() == "" {
		t.Fatal("session paths empty")
	}
	c.CompactRatio()
	c.ContextSnapshot()
	if c.LastUsage() != nil {
		t.Fatal("usage before turn")
	}
	hit, miss := c.SessionCache()
	if hit != 0 || miss != 0 {
		t.Fatalf("cache = %d/%d", hit, miss)
	}
	if bal, err := c.Balance(context.Background()); err != nil || bal != nil {
		t.Fatalf("balance = %v err=%v", bal, err)
	}
	if c.Checkpoints() == nil {
		t.Fatal("expected non-nil checkpoint list")
	}
	if c.CheckpointHasBoundary(0) {
		t.Fatal("unexpected boundary")
	}
	if c.RuntimeHub() != nil {
		t.Fatal("expected nil runtime hub")
	}
}

func TestControllerRunCancelTurn(t *testing.T) {
	sess := agent.NewSession("sys")
	exec := agent.New(&fakeCtrlProvider{reply: "ok"}, tool.NewRegistry(), sess, agent.Options{}, event.Discard)
	c := New(Options{Runner: exec, Executor: exec, Sink: event.Discard, Hooks: hook.NewRunner(nil, "", nil, nil)})
	if c.Turn() != 0 {
		t.Fatalf("turn = %d", c.Turn())
	}
	if c.Running() {
		t.Fatal("should not be running")
	}
	if err := c.Run(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if len(c.History()) < 2 {
		t.Fatalf("history = %d", len(c.History()))
	}
	c.Cancel() // no-op without active turn
}

func TestControllerManagementNotices(t *testing.T) {
	sink := &ctrlRecordSink{}
	sess := agent.NewSession("system prompt")
	exec := agent.New(nil, tool.NewRegistry(), sess, agent.Options{}, event.Discard)
	mem := &memory.Set{Docs: []memory.Source{{Scope: "project", Path: "ARCDESK.md"}}}
	skills := []skill.Skill{{Name: "explore", Description: "explore repo", RunAs: skill.RunSubagent}}
	c := New(Options{
		Runner:       exec,
		Executor:     exec,
		Sink:         sink,
		Label:        "test",
		SystemPrompt: "system prompt",
		Skills:       skills,
		AllSkills:    skills,
		Memory:       mem,
		Hooks:        hook.NewRunner(nil, "", nil, nil),
	})
	c.managementNotice("/memory")
	if !strings.Contains(sink.joined(), "ARCDESK.md") {
		t.Fatalf("memory notice = %q", sink.joined())
	}
	c.managementNotice("/model")
	if !strings.Contains(sink.joined(), "model") {
		t.Fatalf("model notice = %q", sink.joined())
	}
	c.managementNotice("/skills")
	if !strings.Contains(sink.joined(), "explore") {
		t.Fatalf("skills notice = %q", sink.joined())
	}
	c.managementNotice("/hooks")
	if sink.joined() == "" {
		t.Fatal("expected hooks notice")
	}
	if c.managementNotice("/unknown") {
		t.Fatal("unknown command should not be handled")
	}
}

func TestControllerCompactAndPlanMode(t *testing.T) {
	fp := &fakeCtrlProvider{reply: "summary"}
	sess := agent.NewSession("sys")
	for i := 0; i < 6; i++ {
		sess.Add(provider.Message{Role: provider.RoleUser, Content: strings.Repeat("x", 40)})
		sess.Add(provider.Message{Role: provider.RoleAssistant, Content: strings.Repeat("y", 40)})
	}
	exec := agent.New(fp, tool.NewRegistry(), sess, agent.Options{
		ContextWindow: 500,
		CompactRatio:  0.5,
	}, event.Discard)
	c := New(Options{Executor: exec, Runner: exec, Sink: event.Discard, Hooks: hook.NewRunner(nil, "", nil, nil)})
	if err := c.Compact(context.Background(), "focus tests"); err != nil {
		t.Fatal(err)
	}
	c.SetPlanMode(true)
	if !c.PlanMode() {
		t.Fatal("plan mode")
	}
	c.SetAutoPlan("on")
}

func TestSlashLanguageAndAutoPlanItems(t *testing.T) {
	if items := languageArgItems(nil); len(items) != 3 {
		t.Fatalf("language items = %d", len(items))
	}
	if items := autoPlanArgItems(nil); len(items) != 2 {
		t.Fatalf("autoplan items = %d", len(items))
	}
	items, _ := SlashArgItems("/language ", ArgData{})
	if len(items) == 0 {
		t.Fatal("expected /language items")
	}
}

func TestControllerRegisterSessionTool(t *testing.T) {
	c := testController(t)
	c.RegisterSessionTool(fakeControlTool{name: "ui_tool"})
	c.RegisterSessionTool(nil)
	var nilCtrl *Controller
	nilCtrl.RegisterSessionTool(fakeControlTool{name: "x"})
}

func TestControllerSkillEnabled(t *testing.T) {
	c := testController(t)
	if !c.SkillEnabled("explore") {
		t.Fatal("explore should be enabled")
	}
	if len(c.DisabledSkills()) != 0 {
		t.Fatalf("disabled = %v", c.DisabledSkills())
	}
}

func TestControllerEnableInteractiveApproval(t *testing.T) {
	c := testController(t)
	c.EnableInteractiveApproval()
	c.EnableDesktopSubagentGate()
}

type fakeCtrlProvider struct {
	reply string
}

func (f *fakeCtrlProvider) Name() string { return "fake" }

func (f *fakeCtrlProvider) Stream(_ context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 3)
	ch <- provider.Chunk{Type: provider.ChunkText, Text: f.reply}
	ch <- provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{PromptTokens: 42, TotalTokens: 50}}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func TestControllerSubmitManagementCommand(t *testing.T) {
	sink := &ctrlRecordSink{}
	sess := agent.NewSession("sys")
	exec := agent.New(nil, tool.NewRegistry(), sess, agent.Options{}, event.Discard)
	mem := &memory.Set{Docs: []memory.Source{{Scope: "project", Path: "notes.md"}}}
	c := New(Options{
		Runner: exec, Executor: exec, Sink: sink, Memory: mem,
		Hooks: hook.NewRunner(nil, "", nil, nil),
	})
	c.Submit("/memory")
	time.Sleep(50 * time.Millisecond)
	if !strings.Contains(sink.joined(), "notes.md") {
		t.Fatalf("got %q", sink.joined())
	}
}

func TestControllerNewSessionRotates(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	exec := agent.New(nil, tool.NewRegistry(), sess, agent.Options{}, event.Discard)
	c := New(Options{
		Runner:     exec,
		Executor:   exec,
		Sink:       event.Discard,
		SessionDir: dir,
		Hooks:      hook.NewRunner(nil, "", nil, nil),
	})
	c.SetSessionPath(filepath.Join(dir, "session.jsonl"))
	if err := c.NewSession(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectSymlinkComponentsSafe(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".arcdesk", "attachments")
	okDir := filepath.Join(root, "ok")
	if err := os.MkdirAll(okDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(okDir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rejectSymlinkComponents(file, root); err != nil {
		t.Fatal(err)
	}
}

func TestControllerContextSnapshotAfterUsage(t *testing.T) {
	sess := agent.NewSession("sys")
	fp := &fakeCtrlProvider{reply: "hi"}
	exec := agent.New(fp, tool.NewRegistry(), sess, agent.Options{ContextWindow: 1000}, event.Discard)
	c := New(Options{Runner: exec, Executor: exec, Sink: event.Discard, Hooks: hook.NewRunner(nil, "", nil, nil)})
	if err := c.Run(context.Background(), "ping"); err != nil {
		t.Fatal(err)
	}
	prompt, window := c.ContextSnapshot()
	if prompt != 42 || window != 1000 {
		t.Fatalf("snapshot = %d/%d", prompt, window)
	}
	if c.LastUsage() == nil {
		t.Fatal("expected usage")
	}
}
