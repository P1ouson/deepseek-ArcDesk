package control

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/agent"
	"arcdesk/internal/command"
	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/hook"
	"arcdesk/internal/memory"
	"arcdesk/internal/plugin"
	"arcdesk/internal/provider"
	"arcdesk/internal/skill"
	"arcdesk/internal/tool"
)

func TestControllerSubmitManagementAndSessionOps(t *testing.T) {
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
	if err := os.WriteFile("ARCDESK.md", []byte("project notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	mem := memory.Load(memory.Options{CWD: dir, UserDir: filepath.Join(home, ".config", "arcdesk")})
	sk := skill.Skill{Name: "explore", Body: "task {{task}}", Scope: skill.ScopeBuiltin}
	sink := &ctrlRecordSink{}
	sess := agent.NewSession("sys")
	fp := &fakeCtrlProvider{reply: "ok"}
	exec := agent.New(fp, nil, sess, agent.Options{ContextWindow: 8000, CompactRatio: 0.5}, event.Discard)
	c := New(Options{
		Runner:     exec,
		Executor:   exec,
		Sink:       sink,
		Memory:     mem,
		Skills:     []skill.Skill{sk},
		AllSkills:  []skill.Skill{sk},
		Commands:   []command.Command{{Name: "review", Body: "review $ARGUMENTS"}},
		Hooks:      hook.NewRunner(hook.Load(hook.LoadOptions{ProjectRoot: dir, Trusted: true}), dir, nil, nil),
		SessionDir: dir,
		Label:      "test",
	})
	c.SetSessionPath(agent.NewSessionPath(dir, "test"))
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}

	for _, cmd := range []string{"/model", "/memory", "/skills", "/hooks", "/tree"} {
		c.Submit(cmd)
	}
	if joined := sink.joined(); !strings.Contains(joined, "ARCDESK.md") {
		t.Fatalf("missing memory listing: %q", joined)
	}

	c.Submit("/skills disable explore")
	c.Submit("/skills enable explore")
	c.Submit("/compact focus")
	time.Sleep(100 * time.Millisecond)
	c.Submit("/new")
	time.Sleep(100 * time.Millisecond)

	c.Submit("!echo hi")
	time.Sleep(200 * time.Millisecond)

	c.Submit("/review the diff")
	time.Sleep(100 * time.Millisecond)

	c.Submit("/explore bugs")
	time.Sleep(100 * time.Millisecond)

	c.Submit("@ARCDESK.md")
	time.Sleep(100 * time.Millisecond)

	c.Submit("# quick note")
	time.Sleep(50 * time.Millisecond)
	if !strings.Contains(sink.joined(), "remembered") {
		t.Fatal("expected quick remember notice")
	}
}

func TestControllerAskBypassAndImageDataURL(t *testing.T) {
	t.Chdir(t.TempDir())
	path, err := SaveImageDataURL("data:image/png;base64," + tinyPNG)
	if err != nil {
		t.Fatal(err)
	}
	url, err := ImageDataURL(path)
	if err != nil || !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("url=%q err=%v", url, err)
	}

	c := New(Options{Sink: event.Discard})
	c.SetBypass(true)
	ans, err := c.Ask(context.Background(), []event.AskQuestion{{ID: "q", Options: []event.AskOption{{Label: "a"}}}})
	if err != nil || len(ans) != 1 {
		t.Fatalf("Ask bypass = %v err=%v", ans, err)
	}

	c2 := New(Options{Sink: event.Discard})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c2.Ask(ctx, []event.AskQuestion{{ID: "q"}}); err == nil {
		t.Fatal("expected cancelled Ask")
	}
}

func TestControllerMCPPromptSubmit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	reg := tool.NewRegistry()
	c := New(Options{
		Host:      plugin.NewHost(),
		Registry:  reg,
		PluginCtx: ctx,
		Runner:    &fakeTurnRunner{},
		Executor:  agent.New(nil, reg, agent.NewSession("sys"), agent.Options{}, event.Discard),
		Sink:      event.Discard,
	})
	defer c.DisconnectMCPServer("mockctrl")
	if _, err := c.ConnectMCPServer(config.PluginEntry{
		Name:    "mockctrl",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env: map[string]string{
			"GO_WANT_HELPER_PROCESS":  "1",
			"GO_WANT_HELPER_PROMPTS":  "1",
		},
	}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

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

func TestControllerAddMCPServerPersists(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile := func(name, body string) {
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("arcdesk.toml", `
default_model = "test-model"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "TEST_KEY"
`)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := New(Options{
		Host:      plugin.NewHost(),
		Registry:  tool.NewRegistry(),
		PluginCtx: ctx,
		Sink:      event.Discard,
	})
	defer c.DisconnectMCPServer("persist-mcp")
	if _, err := c.AddMCPServer(config.PluginEntry{
		Name:    "persist-mcp",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	}); err != nil {
		t.Fatalf("AddMCPServer: %v", err)
	}
	body, err := os.ReadFile("arcdesk.toml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "persist-mcp") {
		t.Fatalf("config = %s", body)
	}
}

func TestControllerResolveBranchAndRefs(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("ref.go", []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "hi"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, Label: "b", Sink: event.Discard})
	c.SetSessionPath(agent.NewSessionPath(dir, "b"))
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Branch("child"); err != nil {
		t.Fatal(err)
	}
	childID := agent.BranchID(c.SessionPath())
	if _, err := c.SwitchBranch(childID); err != nil {
		t.Fatal(err)
	}
	if _, errs := c.ResolveRefs(context.Background(), "@ref.go"); len(errs) != 0 {
		t.Fatalf("errs = %v", errs)
	}
}

func TestControllerBalanceWithMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"is_available": true,
			"balance_infos": []map[string]string{
				{"currency": "USD", "total_balance": "1.00", "granted_balance": "0", "topped_up_balance": "1.00"},
			},
		})
	}))
	defer srv.Close()
	c := New(Options{
		Sink:          event.Discard,
		BalanceURL:    srv.URL,
		BalanceKey:    "key",
		BalanceClient: srv.Client(),
	})
	bal, err := c.Balance(context.Background())
	if err != nil || bal == nil || !bal.Available {
		t.Fatalf("Balance = %+v err=%v", bal, err)
	}
}

func TestControllerSendAndCancel(t *testing.T) {
	sess := agent.NewSession("sys")
	slow := &slowCtrlProvider{}
	exec := agent.New(slow, tool.NewRegistry(), sess, agent.Options{}, event.Discard)
	c := New(Options{Runner: exec, Executor: exec, Sink: event.Discard})
	go func() { c.Send("wait") }()
	time.Sleep(20 * time.Millisecond)
	if !c.Running() {
		t.Fatal("expected running turn")
	}
	c.Cancel()
	time.Sleep(100 * time.Millisecond)
	if c.Running() {
		t.Fatal("expected turn cancelled")
	}
}

func TestControllerSubmitRewind(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "second"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, Label: "rw", Sink: event.Discard})
	c.SetSessionPath(agent.NewSessionPath(dir, "rw"))
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	c.cpBound[0] = 1
	c.mu.Unlock()
	c.Submit("/rewind 0 conversation")
	time.Sleep(100 * time.Millisecond)
}

func TestSaveClipboardImageErrorOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows clipboard test")
	}
	t.Chdir(t.TempDir())
	if _, err := SaveClipboardImage(); err == nil {
		t.Skip("clipboard contained an image")
	}
}

type slowCtrlProvider struct{}

func (s *slowCtrlProvider) Name() string { return "slow" }

func (s *slowCtrlProvider) Stream(ctx context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

func TestControllerGetterAPIsAndRun(t *testing.T) {
	fp := &fakeCtrlProvider{reply: "done"}
	sess := agent.NewSession("sys")
	exec := agent.New(fp, tool.NewRegistry(), sess, agent.Options{ContextWindow: 500, CompactRatio: 0.5}, event.Discard)
	sk := skill.Skill{Name: "explore"}
	c := New(Options{
		Runner:      exec,
		Executor:    exec,
		Sink:        event.Discard,
		Label:       "label",
		AllSkills:   []skill.Skill{sk},
		Skills:      []skill.Skill{sk},
		RuntimeHub:  nil,
	})
	if err := c.Run(context.Background(), "ping"); err != nil {
		t.Fatal(err)
	}
	if c.CompactRatio() != 0.5 {
		t.Fatalf("ratio = %v", c.CompactRatio())
	}
	c.LastUsage()
	c.SessionCache()
	c.Checkpoints()
	if got := c.AllSkills(); len(got) != 1 {
		t.Fatalf("AllSkills = %v", got)
	}
	c.DisabledSkills()
	if err := c.Compact(context.Background(), "focus"); err != nil {
		t.Fatal(err)
	}
	reg := tool.NewRegistry()
	c2 := New(Options{Registry: reg, Host: plugin.NewHost()})
	c2.RemoveMCPServer("missing")
	if ok := c2.DisconnectMCPServer("missing"); ok {
		t.Fatal("disconnect missing should be false")
	}
}
