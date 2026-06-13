package control

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/agent"
	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/hook"
	"arcdesk/internal/jobs"
	"arcdesk/internal/memory"
	"arcdesk/internal/plugin"
	"arcdesk/internal/provider"
	"arcdesk/internal/skill"
	"arcdesk/internal/tool"
)

func isolateControlHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", filepath.Join(dir, "AppData"))
	return dir
}

func TestControllerAnswerQuestion(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	ch := make(chan []event.AskAnswer, 1)
	c.mu.Lock()
	c.asks["q1"] = ch
	c.mu.Unlock()
	answers := []event.AskAnswer{{QuestionID: "q1", Selected: []string{"yes"}}}
	c.AnswerQuestion("q1", answers)
	select {
	case got := <-ch:
		if len(got) != 1 || got[0].QuestionID != "q1" {
			t.Fatalf("got %+v", got)
		}
	default:
		t.Fatal("expected answer on channel")
	}
	c.AnswerQuestion("missing", answers) // ignored
}

func TestControllerForkAndForkSession(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "u1"})
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "a1"})
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "u2"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, Label: "test", Sink: event.Discard})
	c.SetSessionPath(agent.NewSessionPath(dir, "test"))
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	rootPath := c.SessionPath()
	c.mu.Lock()
	c.cpBound[1] = 3
	c.mu.Unlock()

	forkPath, err := c.Fork(1)
	if err != nil {
		t.Fatal(err)
	}
	if forkPath == rootPath || len(c.History()) != 3 {
		t.Fatalf("fork path=%q hist=%d", forkPath, len(c.History()))
	}

	exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "back"})
	c.mu.Lock()
	c.cpBound[1] = 3
	c.mu.Unlock()
	sidePath, err := c.ForkSession(1, "side")
	if err != nil {
		t.Fatal(err)
	}
	if sidePath == c.SessionPath() {
		t.Fatal("ForkSession should not switch controller")
	}
	meta, ok, err := agent.LoadBranchMeta(sidePath)
	if err != nil || !ok || meta.Name != "side" {
		t.Fatalf("meta ok=%v err=%v meta=%+v", ok, err, meta)
	}
}

func TestControllerSummarizeFromAndUpTo(t *testing.T) {
	fp := &fakeCtrlProvider{reply: "summary text"}
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "world"})
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "more"})
	exec := agent.New(fp, tool.NewRegistry(), sess, agent.Options{}, event.Discard)
	dir := t.TempDir()
	c := New(Options{Executor: exec, SessionDir: dir, Label: "t", Sink: event.Discard})
	c.SetSessionPath(agent.NewSessionPath(dir, "t"))
	c.mu.Lock()
	c.cpBound[0] = 3
	c.cpBound[1] = 3
	c.mu.Unlock()

	if err := c.SummarizeUpTo(context.Background(), 0); err != nil {
		t.Fatalf("SummarizeUpTo: %v", err)
	}
	if len(c.History()) != 3 {
		t.Fatalf("after SummarizeUpTo hist=%d", len(c.History()))
	}

	sess2 := agent.NewSession("sys")
	sess2.Add(provider.Message{Role: provider.RoleUser, Content: "a"})
	sess2.Add(provider.Message{Role: provider.RoleAssistant, Content: "b"})
	sess2.Add(provider.Message{Role: provider.RoleUser, Content: "c"})
	exec2 := agent.New(fp, tool.NewRegistry(), sess2, agent.Options{}, event.Discard)
	c2 := New(Options{Executor: exec2, SessionDir: dir, Label: "t2", Sink: event.Discard})
	c2.mu.Lock()
	c2.cpBound[1] = 3
	c2.mu.Unlock()
	if err := c2.SummarizeFrom(context.Background(), 1); err != nil {
		t.Fatalf("SummarizeFrom: %v", err)
	}
	if len(c2.History()) != 4 {
		t.Fatalf("after SummarizeFrom hist=%d", len(c2.History()))
	}
}

func TestControllerResume(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resume.jsonl")
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "loaded"})
	exec := agent.New(nil, nil, agent.NewSession("other"), agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, Sink: event.Discard})
	c.Resume(sess, path)
	if c.SessionPath() != path {
		t.Fatalf("path = %q", c.SessionPath())
	}
	if len(c.History()) != 2 {
		t.Fatalf("history = %d", len(c.History()))
	}
}

func TestControllerLabelCloseJobs(t *testing.T) {
	jm := jobs.NewManager(event.Discard)
	c := New(Options{
		Label: "deepseek-flash",
		Jobs:  jm,
		Sink:  event.Discard,
		Hooks: hook.NewRunner(nil, "", nil, nil),
	})
	c.mu.Lock()
	c.startedOnce = true
	cleanupCalled := false
	c.cleanup = func() { cleanupCalled = true }
	c.mu.Unlock()
	if c.Label() != "deepseek-flash" {
		t.Fatalf("label = %q", c.Label())
	}
	if jobs := c.Jobs(); jobs != nil && len(jobs) != 0 {
		t.Fatalf("jobs = %v", jobs)
	}
	c.Close()
	if !cleanupCalled {
		t.Fatal("cleanup not called")
	}
}

func TestControllerRunSkillAndHasRefs(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("foo.go", []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	sk := skill.Skill{Name: "explore", Body: "Explore {{task}}"}
	c := New(Options{Skills: []skill.Skill{sk}, Sink: event.Discard})
	sent, found := c.RunSkill("/explore find bugs")
	if !found || !strings.Contains(sent, "find bugs") {
		t.Fatalf("RunSkill = (%q,%v)", sent, found)
	}
	if _, found := c.RunSkill("/missing"); found {
		t.Fatal("missing skill should not match")
	}
	if !c.HasRefs("@foo.go") {
		t.Fatal("expected @foo.go ref")
	}
	if c.HasRefs("plain text") {
		t.Fatal("plain text should have no refs")
	}
}

func TestControllerSaveDocForgetMemoryRememberNote(t *testing.T) {
	home := isolateControlHome(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("ARCDESK.md", []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	mem := memory.Load(memory.Options{CWD: dir, UserDir: filepath.Join(home, ".config", "arcdesk")})
	c := New(Options{Memory: mem, Sink: event.Discard, Hooks: hook.NewRunner(nil, "", nil, nil)})

	written, err := c.SaveDoc("ARCDESK.md", "new body")
	if err != nil || written == "" {
		t.Fatalf("SaveDoc = %q err=%v", written, err)
	}
	if path, err := c.QuickAdd(memory.ScopeProject, "note one"); err != nil || path == "" {
		t.Fatalf("QuickAdd = %q err=%v", path, err)
	}
	c.rememberProjectNote("")
	var sink ctrlRecordSink
	c2 := New(Options{Memory: mem, Sink: &sink, Hooks: hook.NewRunner(nil, "", nil, nil)})
	c2.rememberProjectNote("saved fact")
	if !strings.Contains(sink.joined(), "remembered") {
		t.Fatalf("notice = %q", sink.joined())
	}

	if _, err := mem.Store.Save(memory.Memory{Name: "temp-mem", Description: "d", Type: memory.TypeProject, Body: "fact"}); err != nil {
		t.Fatal(err)
	}
	c3 := New(Options{Memory: memory.Load(memory.Options{CWD: dir, UserDir: filepath.Join(home, ".config", "arcdesk")}), Sink: event.Discard})
	if err := c3.ForgetMemory("temp-mem"); err != nil {
		t.Fatal(err)
	}
}

func TestControllerSetSkillEnabled(t *testing.T) {
	isolateControlHome(t)
	sk := skill.Skill{Name: "explore", Scope: skill.ScopeBuiltin}
	c := New(Options{AllSkills: []skill.Skill{sk}, Skills: []skill.Skill{sk}})
	if err := c.SetSkillEnabled("explore", false); err != nil {
		t.Fatal(err)
	}
	if c.SkillEnabled("explore") {
		t.Fatal("skill should be disabled")
	}
	if err := c.SetSkillEnabled("missing", true); err == nil {
		t.Fatal("expected unknown skill error")
	}
}

func TestControllerMCPConnectAndList(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("arcdesk.toml", []byte(`
[[plugins]]
name = "mockctrl"
command = "mock"
tier = "lazy"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := tool.NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := New(Options{
		Host:       plugin.NewHost(),
		Registry:   reg,
		PluginCtx:  ctx,
		Sink:       event.Discard,
	})
	defer func() {
		c.DisconnectMCPServer("mockctrl")
	}()
	n, err := c.ConnectMCPServer(config.PluginEntry{
		Name:    "mockctrl",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	})
	if err != nil {
		t.Fatalf("ConnectMCPServer: %v", err)
	}
	if n == 0 {
		t.Fatal("expected tools from mock MCP")
	}
	if names := c.DisconnectedMCPNames(); len(names) != 0 {
		t.Fatalf("disconnected = %v", names)
	}

	var listed []string
	c4 := New(Options{
		Host: c.Host(),
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				listed = append(listed, e.Text)
			}
		}),
	})
	c4.Submit("/mcp")
	if !strings.Contains(strings.Join(listed, "\n"), "mockctrl") {
		t.Fatalf("mcp list = %v", listed)
	}
}

func TestControllerBalanceNoURL(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	bal, err := c.Balance(context.Background())
	if err != nil || bal != nil {
		t.Fatalf("Balance = %v err=%v", bal, err)
	}
}

func TestControllerConnectConfiguredMCPServerMissing(t *testing.T) {
	isolateControlHome(t)
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

[[plugins]]
name = "lazy-one"
command = "noop"
tier = "lazy"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New(Options{Sink: event.Discard, Host: plugin.NewHost()})
	if _, err := c.ConnectConfiguredMCPServer("missing"); err == nil {
		t.Fatal("expected error for missing server")
	}
	if names := c.DisconnectedMCPNames(); len(names) != 1 || names[0] != "lazy-one" {
		t.Fatalf("disconnected = %v", names)
	}
}

// TestHelperProcess is a minimal stdio MCP server for control package tests.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	in := bufio.NewReader(os.Stdin)
	for {
		line, err := in.ReadBytes('\n')
		if err != nil {
			return
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var req struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(line, &req); err != nil || req.ID == nil {
			continue
		}
		var result any
		switch req.Method {
		case "initialize":
			caps := map[string]any{}
			if os.Getenv("GO_WANT_HELPER_PROMPTS") == "1" {
				caps["prompts"] = map[string]any{}
			}
			if os.Getenv("GO_WANT_HELPER_RESOURCES") == "1" {
				caps["resources"] = map[string]any{}
			}
			result = map[string]any{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]any{"name": "mock", "version": "0"},
				"capabilities":    caps,
			}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{{
				"name":        "echo",
				"description": "echo",
				"inputSchema": map[string]any{"type": "object"},
			}}}
		case "prompts/list":
			result = map[string]any{"prompts": []map[string]any{{
				"name": "hello",
				"arguments": []map[string]any{{"name": "name"}},
			}}}
		case "prompts/get":
			result = map[string]any{
				"messages": []map[string]any{{
					"role": "user",
					"content": map[string]any{"type": "text", "text": "hello prompt"},
				}},
			}
		case "resources/list":
			result = map[string]any{"resources": []map[string]any{{
				"uri": "doc://test", "name": "test", "mimeType": "text/plain",
			}}}
		case "resources/read":
			result = map[string]any{"contents": []map[string]any{{
				"uri": "doc://test", "mimeType": "text/plain", "text": "resource body",
			}}}
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": *req.ID, "result": result}
		b, _ := json.Marshal(resp)
		os.Stdout.Write(append(b, '\n'))
	}
}
