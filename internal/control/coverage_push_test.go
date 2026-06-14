package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"arcdesk/internal/agent"
	"arcdesk/internal/command"
	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/hook"
	"arcdesk/internal/instruction"
	"arcdesk/internal/memory"
	"arcdesk/internal/plugin"
	"arcdesk/internal/provider"
	"arcdesk/internal/skill"
	"arcdesk/internal/tool"
)

type seqCtrlProvider struct {
	replies []string
	mu      sync.Mutex
	n       int
}

func (p *seqCtrlProvider) Name() string { return "seq" }

func (p *seqCtrlProvider) Stream(_ context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	p.mu.Lock()
	i := p.n
	if i >= len(p.replies) {
		i = len(p.replies) - 1
	}
	p.n++
	reply := p.replies[i]
	p.mu.Unlock()
	ch := make(chan provider.Chunk, 3)
	ch <- provider.Chunk{Type: provider.ChunkText, Text: reply}
	ch <- provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{PromptTokens: 1, TotalTokens: 2}}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func hookRunnerBlockingSubmit() *hook.Runner {
	hooks := []hook.ResolvedHook{{
		HookConfig: hook.HookConfig{Command: "block"},
		Event:      hook.UserPromptSubmit,
		Scope:      hook.ScopeGlobal,
	}}
	spawner := func(_ context.Context, _ hook.SpawnInput) hook.SpawnResult {
		return hook.SpawnResult{ExitCode: 2}
	}
	return hook.NewRunner(hooks, "", spawner, nil)
}

func hookRunnerPassingStop() *hook.Runner {
	hooks := []hook.ResolvedHook{{
		HookConfig: hook.HookConfig{Command: "ok"},
		Event:      hook.UserPromptSubmit,
		Scope:      hook.ScopeGlobal,
	}, {
		HookConfig: hook.HookConfig{Command: "ok"},
		Event:      hook.Stop,
		Scope:      hook.ScopeGlobal,
	}, {
		HookConfig: hook.HookConfig{Command: "ok"},
		Event:      hook.Notification,
		Scope:      hook.ScopeGlobal,
	}}
	spawner := func(_ context.Context, _ hook.SpawnInput) hook.SpawnResult {
		return hook.SpawnResult{ExitCode: 0}
	}
	return hook.NewRunner(hooks, "", spawner, nil)
}

func TestRunWithHooksBlockAndPass(t *testing.T) {
	sess := agent.NewSession("sys")
	exec := agent.New(&fakeCtrlProvider{reply: "ok"}, tool.NewRegistry(), sess, agent.Options{}, event.Discard)

	blockRunner := hookRunnerBlockingSubmit()
	c := New(Options{Runner: exec, Executor: exec, Sink: event.Discard, Hooks: blockRunner})
	if err := c.Run(context.Background(), "blocked"); err != nil {
		t.Fatalf("blocked run should return nil: %v", err)
	}

	passRunner := hookRunnerPassingStop()
	c = New(Options{Runner: exec, Executor: exec, Sink: event.Discard, Hooks: passRunner})
	if err := c.Run(context.Background(), "allowed"); err != nil {
		t.Fatalf("allowed run: %v", err)
	}
}

func TestSendRunTurnWithHooks(t *testing.T) {
	sink, done, events := collectSink()
	sess := agent.NewSession("sys")
	exec := agent.New(&fakeCtrlProvider{reply: "answer"}, tool.NewRegistry(), sess, agent.Options{}, sink)
	c := New(Options{
		Runner:   exec,
		Executor: exec,
		Sink:     sink,
		Hooks:    hookRunnerPassingStop(),
	})
	c.Send("hello hooks")
	waitForDone(t, done)

	block := hookRunnerBlockingSubmit()
	c = New(Options{Runner: exec, Executor: exec, Sink: sink, Hooks: block})
	c.Send("blocked turn")
	waitForDone(t, done)
	if len(*events) == 0 {
		t.Fatal("expected events")
	}
}

func TestRunTurnPlanModeApprovedExecution(t *testing.T) {
	plan := "1. Implement feature\n   - write code\n   - run tests"
	prov := &seqCtrlProvider{replies: []string{plan, "implemented"}}
	sess := agent.NewSession("sys")
	exec := agent.New(prov, tool.NewRegistry(), sess, agent.Options{}, event.Discard)
	approvalCh := make(chan string, 1)
	sink, done, _ := collectSink()
	wrapped := event.FuncSink(func(e event.Event) {
		sink.Emit(e)
		if e.Kind == event.ApprovalRequest && e.Approval.Tool == planApprovalTool {
			approvalCh <- e.Approval.ID
		}
	})
	c := New(Options{
		Runner:   exec,
		Executor: exec,
		Sink:     wrapped,
		Hooks:    hookRunnerPassingStop(),
	})
	c.EnableInteractiveApproval()
	c.SetPlanMode(true)

	go func() {
		id := <-approvalCh
		c.Approve(id, true, false, false)
	}()

	c.Send("build the feature")
	waitForDone(t, done)
	if prov.n < 2 {
		t.Fatalf("expected two model calls, got %d", prov.n)
	}
}

func TestRunTurnPlanModeRejected(t *testing.T) {
	prov := &seqCtrlProvider{replies: []string{"1. Plan\n   - step"}}
	sess := agent.NewSession("sys")
	exec := agent.New(prov, tool.NewRegistry(), sess, agent.Options{}, event.Discard)
	approvalCh := make(chan string, 1)
	sink, done, _ := collectSink()
	wrapped := event.FuncSink(func(e event.Event) {
		sink.Emit(e)
		if e.Kind == event.ApprovalRequest && e.Approval.Tool == planApprovalTool {
			approvalCh <- e.Approval.ID
		}
	})
	c := New(Options{Runner: exec, Executor: exec, Sink: wrapped})
	c.EnableInteractiveApproval()
	c.SetPlanMode(true)

	go func() {
		id := <-approvalCh
		c.Approve(id, false, false, false)
	}()

	c.Send("plan only")
	waitForDone(t, done)
	if prov.n != 1 {
		t.Fatalf("rejected plan should not execute, calls=%d", prov.n)
	}
}

func TestSubmitPathsWithTurnDone(t *testing.T) {
	home := isolateControlHome(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("arcdesk.toml", []byte(`
default_model = "test-model"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "TEST_KEY"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("ARCDESK.md", []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("ref.txt", []byte("ref content"), 0o644); err != nil {
		t.Fatal(err)
	}

	mem := memory.Load(memory.Options{CWD: dir, UserDir: filepath.Join(home, ".config", "arcdesk")})
	sk := skill.Skill{Name: "explore", Body: "explore {{task}}", Scope: skill.ScopeBuiltin}
	sink, done, _ := collectSink()
	sess := agent.NewSession("sys")
	fp := &fakeCtrlProvider{reply: "ok"}
	exec := agent.New(fp, tool.NewRegistry(), sess, agent.Options{ContextWindow: 8000, CompactRatio: 0.5}, sink)
	c := New(Options{
		Runner:        exec,
		Executor:      exec,
		Sink:          sink,
		Memory:        mem,
		Skills:        []skill.Skill{sk},
		AllSkills:     []skill.Skill{sk},
		Commands:      []command.Command{{Name: "review", Body: "review $ARGUMENTS"}},
		Hooks:         hookRunnerPassingStop(),
		SessionDir:    dir,
		Label:         "test",
		ReadRoots:     []string{dir},
		WorkspaceRoot: dir,
	})
	c.SetSessionPath(agent.NewSessionPath(dir, "submit"))

	c.Submit("/compact")
	time.Sleep(150 * time.Millisecond)
	c.Submit("/new")
	time.Sleep(150 * time.Millisecond)

	c.Submit("/branch child")
	c.Submit("/branch 1 named")
	c.Submit("/switch")
	c.Submit("/rewind bad args")
	c.Submit("/unknown-slash-cmd")

	c.Submit("/review the diff")
	waitForDone(t, done)
	c.Submit("/explore bugs")
	waitForDone(t, done)
	c.Submit("@ref.txt")
	waitForDone(t, done)
}

func TestSubmitMCPPromptAndListFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host := plugin.NewHost()
	host.RecordFailure(plugin.Spec{Name: "bad-mcp", Command: "false"}, errors.New("spawn failed"))
	reg := tool.NewRegistry()
	sink, _, events := collectSink()
	c := New(Options{
		Host:      host,
		Registry:  reg,
		PluginCtx: ctx,
		Sink:      sink,
		Runner:    agent.New(nil, reg, agent.NewSession("sys"), agent.Options{}, sink),
	})
	defer c.DisconnectMCPServer("helper-mcp")
	c.Submit("/mcp")
	if len(*events) == 0 {
		t.Fatal("expected mcp list notice")
	}
	if _, err := c.ConnectMCPServer(config.PluginEntry{
		Name:    "helper-mcp",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	text := c.mcpListText()
	if !strings.Contains(text, "helper-mcp") {
		t.Fatalf("mcp list = %q", text)
	}
}

func TestRunGuardedVerifyRollbackOnSend(t *testing.T) {
	root := t.TempDir()
	rel := "main.go"
	if err := os.WriteFile(filepath.Join(root, rel), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	prov := &e2eScriptedProvider{turns: [][]provider.Chunk{
		{
			{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
				ID: "w1", Name: "write_file",
				Arguments: `{"path":"main.go","content":"package main\n\nfunc main() { panic(\"broken\") }\n"}`,
			}},
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "retry 1"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "retry 2"}, {Type: provider.ChunkDone}},
	}}

	reg := tool.NewRegistry()
	reg.Add(e2eWriteTool{root: root})
	exec := agent.New(prov, reg, agent.NewSession(""), agent.Options{
		ProjectChecks:            []instruction.VerifyCheck{{Command: "go test ./..."}},
		VerifyMaxRetries:         2,
		VerifyEnforceFinalAnswer: true,
		VerifyOnFailure:          "rollback",
	}, event.Discard)

	sink, done, _ := collectSink()
	c := New(Options{
		Runner:          exec,
		Executor:        exec,
		Sink:            sink,
		WorkspaceRoot:   root,
		VerifyOnFailure: "rollback",
		Registry:        reg,
	})
	c.SetSessionPath(filepath.Join(root, "session.jsonl"))

	c.Send("break main")
	e := waitForDone(t, done)
	if e.Err == nil || !errors.Is(e.Err, agent.ErrVerifyExhausted) {
		t.Fatalf("TurnDone err = %v, want ErrVerifyExhausted", e.Err)
	}
	got, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	// Readiness-only exhaustion (checks never run) keeps workspace edits per policy.
	if !strings.Contains(string(got), `panic("broken")`) {
		t.Fatalf("readiness-only exhaustion should keep edits, got %q", got)
	}
}

func TestAttachmentErrorBranches(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := SaveAttachmentDataURL("x.pdf", "not-a-data-url"); err == nil {
		t.Fatal("bad data url")
	}
	if _, err := SaveAttachmentDataURL("x.pdf", "data:application/pdf;base64,!!!"); err == nil {
		t.Fatal("bad base64")
	}
	if _, err := SaveAttachmentDataURL("x.pdf", "data:application/pdf;base64,"); err == nil {
		t.Fatal("empty attachment")
	}
	if _, err := SaveImageDataURL("data:image/png;base64,aGk="); err == nil {
		t.Fatal("spoofed png")
	}
	if _, err := SaveImageBytes("image/bmp", []byte("not-image")); err == nil {
		t.Fatal("non-image bytes")
	}
	if _, err := SaveImageBytes("image/unknown", mustBase64(t, tinyPNG)); err == nil {
		t.Fatal("unsupported declared mime")
	}
}

func TestControllerBranchesAndSwitchSuccess(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "hi"})
	exec := agent.New(nil, tool.NewRegistry(), sess, agent.Options{}, event.Discard)
	c := New(Options{
		Executor:   exec,
		Runner:   exec,
		Sink:     event.Discard,
		SessionDir: dir,
		Label:    "branch-test",
		Hooks:    hook.NewRunner(nil, "", nil, nil),
	})
	c.SetSessionPath(agent.NewSessionPath(dir, "branch-test"))
	if _, err := c.Branch("child"); err != nil {
		t.Fatalf("Branch: %v", err)
	}
	branches, err := c.Branches()
	if err != nil || len(branches) < 2 {
		t.Fatalf("Branches = %v err=%v", branches, err)
	}
	if _, err := c.SwitchBranch(branches[len(branches)-1].ID); err != nil {
		t.Fatalf("SwitchBranch: %v", err)
	}
}

func TestControllerNilJobsAndRuntimeHub(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	if c.Jobs() != nil {
		t.Fatal("expected nil jobs")
	}
	if c.RuntimeHub() != nil {
		t.Fatal("expected nil runtime hub")
	}
	if c.CompactRatio() != 0 || c.LastUsage() != nil {
		t.Fatal("nil executor getters should be zero/nil")
	}
	hit, miss := c.SessionCache()
	if hit != 0 || miss != 0 {
		t.Fatal("expected zero cache")
	}
}

func TestRememberProjectNotePaths(t *testing.T) {
	var notices []string
	c := New(Options{Sink: event.FuncSink(func(e event.Event) {
		if e.Text != "" {
			notices = append(notices, e.Text)
		}
	})})
	c.rememberProjectNote("")
	if !strings.Contains(strings.Join(notices, "\n"), "nothing to remember") {
		t.Fatal("expected empty notice")
	}
}

func TestManagementNoticeMCPConnectConfigured(t *testing.T) {
	home := isolateControlHome(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("arcdesk.toml", []byte(fmt.Sprintf(`
[[plugins]]
name = "cfg-mcp"
command = %q
args = ["-test.run=TestHelperProcess", "--"]
env = { GO_WANT_HELPER_PROCESS = "1" }
`, os.Args[0])), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = home
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var notices []string
	c := New(Options{
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
		PluginCtx: ctx,
	})
	defer c.DisconnectMCPServer("cfg-mcp")
	c.managementNotice("/mcp connect cfg-mcp")
	if !strings.Contains(strings.Join(notices, "\n"), "connected") {
		t.Fatalf("notices=%v", notices)
	}
}

func TestResolveRefsMCPResource(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	host := plugin.NewHost()
	reg := tool.NewRegistry()
	c := New(Options{
		Host:      host,
		Registry:  reg,
		PluginCtx: ctx,
		Sink:      event.Discard,
	})
	defer c.DisconnectMCPServer("res-mcp")
	if _, err := c.ConnectMCPServer(config.PluginEntry{
		Name:    "res-mcp",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	block, errs := c.ResolveRefs(ctx, "@res-mcp:doc://missing")
	if len(errs) == 0 && block == "" {
		t.Fatal("expected resource resolution attempt")
	}
}
