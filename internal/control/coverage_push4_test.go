package control

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/agent"
	"arcdesk/internal/command"
	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/plugin"
	"arcdesk/internal/provider"
	"arcdesk/internal/skill"
	"arcdesk/internal/tool"
)

func TestCheckpointsNilStore(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	c.mu.Lock()
	c.cp = nil
	c.mu.Unlock()
	if c.Checkpoints() != nil {
		t.Fatal("nil checkpoint store should return nil")
	}
}

func TestRunCodeReviewGuardPaths(t *testing.T) {
	var nilCtrl *Controller
	if got := nilCtrl.RunCodeReview("standard", "all", nil); got.Err == "" {
		t.Fatal("nil controller")
	}
	busy := New(Options{Registry: tool.NewRegistry(), Sink: event.Discard})
	busy.mu.Lock()
	busy.running = true
	busy.mu.Unlock()
	if got := busy.RunCodeReview("standard", "all", nil); got.Err == "" {
		t.Fatal("busy controller")
	}
	if got := New(Options{Sink: event.Discard}).RunCodeReview("standard", "all", nil); got.Err == "" {
		t.Fatal("nil registry")
	}
	stub := &stubReviewTool{name: "review", out: " "}
	reg := tool.NewRegistry()
	reg.Add(stub)
	if got := New(Options{Registry: reg}).RunCodeReview("standard", "all", nil); got.Err == "" {
		t.Fatal("empty review output")
	}
	stub.err = context.Canceled
	if got := New(Options{Registry: reg}).RunCodeReview("standard", "all", []string{"a.go"}); got.Err == "" {
		t.Fatal("execute error")
	}
}

func TestBuildCodeReviewTaskAllScopes(t *testing.T) {
	for _, scope := range []string{"session", "git", "both", "other", ""} {
		task := BuildCodeReviewTask("standard", scope, []string{"", "x.go"})
		if task == "" {
			t.Fatalf("empty task for scope %q", scope)
		}
	}
}

func TestFormatBranchTreeNested(t *testing.T) {
	branches := []agent.BranchInfo{
		{BranchMeta: agent.BranchMeta{ID: "root", Name: "main line", ParentID: ""}, Turns: 2},
		{BranchMeta: agent.BranchMeta{ID: "child", Name: "experiment", ParentID: "root"}, Turns: 1},
		{BranchMeta: agent.BranchMeta{ID: "orphan", Name: "lost", ParentID: "missing"}, Turns: 3},
	}
	text := FormatBranchTree(branches, "child")
	if !strings.Contains(text, "experiment") || !strings.Contains(text, "current") {
		t.Fatalf("tree = %q", text)
	}
	if got := turnText(1); got != "1 turn" {
		t.Fatalf("turnText = %q", got)
	}
	if got := shortBranchID("20260101-120000.000000000-test"); got == "" {
		t.Fatal("shortBranchID")
	}
}

func TestBranchTreeTextError(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	if text := c.BranchTreeText(); !strings.Contains(text, "branches:") {
		t.Fatalf("text = %q", text)
	}
}

func TestSkillEnabledDisabled(t *testing.T) {
	isolateControlHome(t)
	sk := skill.Skill{Name: "review", Scope: skill.ScopeBuiltin}
	c := New(Options{AllSkills: []skill.Skill{sk}, Skills: []skill.Skill{sk}})
	t.Cleanup(func() {
		cfg := config.LoadForEdit(config.UserConfigPath())
		_ = cfg.SetSkillEnabled("review", true)
		_ = cfg.SaveTo(config.UserConfigPath())
	})
	if err := c.SetSkillEnabled("review", false); err != nil {
		t.Fatal(err)
	}
	if c.SkillEnabled("review") {
		t.Fatal("skill should report disabled")
	}
}

func TestNewSessionDirect(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, Label: "ns", Sink: event.Discard})
	c.SetSessionPath(agent.NewSessionPath(dir, "ns"))
	if err := c.NewSession(); err != nil {
		t.Fatal(err)
	}
	if len(c.History()) != 0 {
		t.Fatalf("history = %d", len(c.History()))
	}
}

func TestSnapshotActivityIfChanged(t *testing.T) {
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, Sink: event.Discard})
	start := c.messageCount()
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "activity"})
	c.snapshotActivityIfChanged(start)
	c.snapshotActivityIfChanged(c.messageCount())
}

func TestMCPPromptWithHelper(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	reg := tool.NewRegistry()
	c := New(Options{
		Host:      plugin.NewHost(),
		Registry:  reg,
		PluginCtx: ctx,
		Sink:      event.Discard,
	})
	defer c.DisconnectMCPServer("prompt-mcp")
	if _, err := c.ConnectMCPServer(config.PluginEntry{
		Name:    "prompt-mcp",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env: map[string]string{
			"GO_WANT_HELPER_PROCESS": "1",
			"GO_WANT_HELPER_PROMPTS": "1",
		},
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	for _, p := range c.Host().Prompts() {
		if strings.Contains(p.Name, "hello") {
			sent, found, err := c.MCPPrompt(ctx, "/"+p.Name+" world")
			if err != nil || !found || sent == "" {
				t.Fatalf("MCPPrompt = (%q,%v,%v)", sent, found, err)
			}
			return
		}
	}
	t.Fatal("no hello prompt registered")
}

func TestRunSkillAndCustomCommand(t *testing.T) {
	sk := skill.Skill{Name: "explore", Description: "explore code", Body: "scan {{args}}"}
	cmd := command.Command{Name: "review", Body: "review {{args}}"}
	c := New(Options{
		Skills:    []skill.Skill{sk},
		AllSkills: []skill.Skill{sk},
		Commands:  []command.Command{cmd},
		Sink:      event.Discard,
	})
	if _, found := c.CustomCommand("/review file.go"); !found {
		t.Fatal("custom command lookup")
	}
	if _, found := c.RunSkill("/explore scan repo"); !found {
		t.Fatal("run skill")
	}
}

func TestResolveRefsFilePath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	name := "sample.txt"
	if err := os.WriteFile(name, []byte("hello refs"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New(Options{Sink: event.Discard, ReadRoots: []string{dir}, WorkspaceRoot: dir})
	block, errs := c.ResolveRefs(context.Background(), "@"+name)
	if len(errs) != 0 || !strings.Contains(block, "hello refs") {
		t.Fatalf("block=%q errs=%v", block, errs)
	}
}

func TestSetShellKillTreeWithProcess(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ping", "-n", "6", "127.0.0.1")
	setShellKillTree(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if cmd.Cancel != nil {
		_ = cmd.Cancel()
	}
	_ = cmd.Wait()
}

func TestComposeWithQueuedMemory(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	c.QueueMemory("project fact added")
	got := c.Compose("do work")
	if !strings.Contains(got, "memory-update") || !strings.Contains(got, "project fact added") {
		t.Fatalf("Compose = %q", got)
	}
}

func TestMcpArgItemsAllBranches(t *testing.T) {
	data := ArgData{
		ServerNames:     []string{"live"},
		ConfiguredMCP:   []string{"live"},
		DisconnectedMCP: []string{"idle"},
	}
	if items := mcpArgItems([]string{"/mcp", "remove", "live", "extra"}, "", data); items != nil {
		t.Fatal("remove with extra args")
	}
	if items := mcpArgItems([]string{"/mcp", "show", "live", "extra"}, "", data); items != nil {
		t.Fatal("show with extra args")
	}
	if items := mcpArgItems([]string{"/mcp", "connect", "idle", "extra"}, "", data); items != nil {
		t.Fatal("connect with extra args")
	}
	if items := mcpArgItems([]string{"/mcp", "add", "--http"}, "--http", data); len(items) == 0 {
		t.Fatal("add flags")
	}
}

func TestModelArgItemsSecondToken(t *testing.T) {
	data := ArgData{ModelRefs: []string{"a/x", "b/y"}, CurrentModel: "a/x"}
	if items := modelArgItems([]string{"/model", "a/x", "extra"}, data); items != nil {
		t.Fatal("model with extra args")
	}
}

func TestResolveRefsMCPResourceSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	host := plugin.NewHost()
	reg := tool.NewRegistry()
	c := New(Options{Host: host, Registry: reg, PluginCtx: ctx, Sink: event.Discard})
	defer c.DisconnectMCPServer("res-mcp")
	if _, err := c.ConnectMCPServer(config.PluginEntry{
		Name:    "res-mcp",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env: map[string]string{
			"GO_WANT_HELPER_PROCESS":   "1",
			"GO_WANT_HELPER_RESOURCES": "1",
		},
	}); err != nil {
		t.Fatal(err)
	}
	host.StartPhaseB(ctx, event.Discard)
	deadline := time.Now().Add(5 * time.Second)
	for len(host.Resources()) == 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	block, errs := c.ResolveRefs(ctx, "@res-mcp:doc://test")
	if len(errs) != 0 || !strings.Contains(block, "resource body") {
		t.Fatalf("block=%q errs=%v", block, errs)
	}
}

func TestSubmitMemoryQuickAdd(t *testing.T) {
	sink := &ctrlRecordSink{}
	c := New(Options{Sink: sink})
	c.Submit("# quick memory note")
	time.Sleep(50 * time.Millisecond)
	if !strings.Contains(sink.joined(), "remembered") {
		t.Fatalf("notices=%v", sink.texts)
	}
}

func TestSaveImageBytesOversized(t *testing.T) {
	t.Chdir(t.TempDir())
	raw := make([]byte, maxImageAttachmentBytes+1)
	raw[0] = 0x89
	raw[1] = 0x50
	if _, err := SaveImageBytes("", raw); err == nil {
		t.Fatal("oversized image should fail")
	}
}
