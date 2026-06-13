package control

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/agent"
	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/hook"
	"arcdesk/internal/memory"
	"arcdesk/internal/plugin"
	"arcdesk/internal/tool"
)

func writeControlCodegraphHelper(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "codegraph-helper")
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	src := filepath.Join(dir, "codegraph-helper.go")
	if err := os.WriteFile(src, []byte(`package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "init" {
		_ = os.MkdirAll(filepath.Join(os.Args[2], ".codegraph"), 0o755)
		return
	}
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
			ID     *int            `+"`json:\"id\"`"+`
			Method string          `+"`json:\"method\"`"+`
			Params json.RawMessage `+"`json:\"params\"`"+`
		}
		if err := json.Unmarshal(line, &req); err != nil || req.ID == nil {
			continue
		}
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]any{"name": "codegraph", "version": "0"},
				"capabilities":    map[string]any{},
			}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{{
				"name": "search", "description": "s", "inputSchema": map[string]any{"type": "object"},
			}}}
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": *req.ID, "result": result}
		b, _ := json.Marshal(resp)
		_, _ = os.Stdout.Write(append(b, '\n'))
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", path, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}
	return path
}

func TestConnectCodegraphMCPServerPaths(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	launcher := writeControlCodegraphHelper(t, dir)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := New(Options{
		Host:      plugin.NewHost(),
		Registry:  tool.NewRegistry(),
		PluginCtx: ctx,
		Sink:      event.Discard,
	})

	if _, err := c.ConnectCodegraphMCPServer(&config.Config{}); err == nil {
		t.Fatal("disabled config should fail")
	}
	cfg := &config.Config{Codegraph: config.CodegraphConfig{Enabled: true, Path: launcher}}
	n, err := c.ConnectCodegraphMCPServer(cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if n == 0 {
		t.Fatal("expected tools")
	}
	defer c.DisconnectMCPServer("codegraph")
}

func TestHookListTextViaSubmit(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll(".arcdesk", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".arcdesk/settings.json", []byte(`{
  "hooks": { "PreToolUse": [ { "match": "bash", "command": "echo x" } ] }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var notices []string
	c := New(Options{
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
		Hooks: hook.NewRunner(hook.Load(hook.LoadOptions{ProjectRoot: dir, Trusted: true}), dir, nil, nil),
	})
	c.Submit("/hooks")
	joined := strings.Join(notices, "\n")
	if !strings.Contains(joined, "PreToolUse") && !strings.Contains(joined, "hook") {
		t.Fatalf("hooks notice = %q", joined)
	}
}

func TestMcpListTextWithFailures(t *testing.T) {
	host := plugin.NewHost()
	host.Failures() // empty initially
	var notices []string
	c := New(Options{
		Host: host,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
	})
	c.Submit("/mcp")
	if len(notices) == 0 {
		t.Fatal("expected mcp list notice")
	}
}

func TestResolveRefsImageAndDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	sub := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	imgPath, err := SaveImageDataURL("data:image/png;base64," + tinyPNG)
	if err != nil {
		t.Fatal(err)
	}
	c := New(Options{Sink: event.Discard, ReadRoots: []string{dir}})
	block, errs := c.ResolveRefs(context.Background(), "@subdir @"+imgPath)
	if len(errs) != 0 {
		t.Fatalf("errs = %v", errs)
	}
	if !strings.Contains(block, "subdir") || !strings.Contains(block, "image") {
		t.Fatalf("block = %q", block)
	}
}

func TestRemoveMCPServerConnected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	reg := tool.NewRegistry()
	c := New(Options{
		Host:      plugin.NewHost(),
		Registry:  reg,
		PluginCtx: ctx,
		Sink:      event.Discard,
	})
	defer c.DisconnectMCPServer("rm-mcp")
	if _, err := c.ConnectMCPServer(config.PluginEntry{
		Name:    "rm-mcp",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	disconnected, err := c.RemoveMCPServer("rm-mcp")
	if err != nil || !disconnected {
		t.Fatalf("remove = %v err=%v", disconnected, err)
	}
}

func TestControllerSeedPlanTodos(t *testing.T) {
	var events []event.Event
	c := New(Options{Sink: event.FuncSink(func(e event.Event) { events = append(events, e) })})
	plan := "- step one\n- step two"
	args := c.seedPlanTodos(plan)
	if args == "" {
		t.Fatal("expected todo args")
	}
	c.completePlanTodos(args)
	if len(events) < 2 {
		t.Fatalf("events = %d", len(events))
	}
	if got := completedPlanTodosJSON("not-json"); got != "" {
		t.Fatal("bad json should yield empty")
	}
}

func TestSubmitRememberAndUnknownSlash(t *testing.T) {
	home := isolateControlHome(t)
	dir := t.TempDir()
	t.Chdir(dir)
	mem := memory.Load(memory.Options{CWD: dir, UserDir: filepath.Join(home, ".config", "arcdesk")})
	var notices []string
	c := New(Options{
		Memory: mem,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
		Hooks: hook.NewRunner(nil, "", nil, nil),
	})
	c.Submit("/remember save this")
	c.Submit("/not-a-real-command")
	joined := strings.Join(notices, "\n")
	if !strings.Contains(joined, "remembered") {
		t.Fatalf("remember = %q", joined)
	}
	if !strings.Contains(joined, "unknown command") {
		t.Fatalf("unknown = %q", joined)
	}
}

func TestControllerRunWithHooks(t *testing.T) {
	dir := t.TempDir()
	runner := &fakeTurnRunner{}
	c := New(Options{
		Runner: runner,
		Sink:   event.Discard,
		Hooks:  hook.NewRunner(hook.Load(hook.LoadOptions{ProjectRoot: dir, Trusted: true}), dir, nil, nil),
	})
	if err := c.Run(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if len(runner.inputs) != 1 {
		t.Fatalf("inputs = %v", runner.inputs)
	}
}

func TestBranchHelperFunctions(t *testing.T) {
	if got, ok := structuredBranchLabel(`{"error":"x"}`); !ok || got != "JSON payload: error" {
		t.Fatalf("error json = %q %v", got, ok)
	}
	if got, ok := structuredBranchLabel("[1,2]"); !ok || got != "JSON array" {
		t.Fatalf("array = %q %v", got, ok)
	}
	if got := oneLineBranch("hello world foo bar baz extra", 10); len(got) > 12 {
		t.Fatalf("truncated = %q", got)
	}
	if got := shortBranchID("20260601-033937.165828000-deepseek-v4-flash"); got == "" {
		t.Fatal("short id empty")
	}
	tree := FormatBranchTree([]agent.BranchInfo{
		{BranchMeta: agent.BranchMeta{ID: "root"}, Preview: "root", Turns: 2},
		{BranchMeta: agent.BranchMeta{ID: "child", ParentID: "root"}, Preview: "child", Turns: 1},
	}, "child")
	if !strings.Contains(tree, "child") {
		t.Fatalf("tree = %q", tree)
	}
}

func TestAttachmentEdgeCases(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := SaveAttachmentDataURL("x.pdf", "not-a-data-url"); err == nil {
		t.Fatal("expected data url error")
	}
	raw := mustBase64(t, tinyPNG)
	if _, err := SaveImageBytes("image/png", raw); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveImageBytes("", raw); err != nil {
		t.Fatal(err)
	}
	if mime := detectedImageMime(raw); mime != "image/png" {
		t.Fatalf("mime = %q", mime)
	}
}

func TestResolveBranchByName(t *testing.T) {
	branches := []agent.BranchInfo{
		{BranchMeta: agent.BranchMeta{ID: "aaa", Name: "experiment"}},
	}
	got, err := resolveBranch(branches, "experiment")
	if err != nil || got.ID != "aaa" {
		t.Fatalf("resolve = %+v err=%v", got, err)
	}
	if _, err := resolveBranch(branches, "missing"); err == nil {
		t.Fatal("expected missing branch error")
	}
}

func TestRunShellQuiet(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	res := c.RunShellQuiet("echo quiet-test")
	if res.Err != "" {
		t.Fatalf("err = %q", res.Err)
	}
	if !strings.Contains(res.Output, "quiet-test") {
		t.Fatalf("out = %q", res.Output)
	}
}

func TestControllerApproveWithPersist(t *testing.T) {
	ids := make(chan string, 1)
	c := New(Options{Sink: event.FuncSink(func(e event.Event) {
		if e.Kind == event.ApprovalRequest {
			ids <- e.Approval.ID
		}
	})})
	go func() {
		id := <-ids
		c.Approve(id, true, false, true)
	}()
	allow, remember, err := gateApprover{c}.Approve(context.Background(), "bash", "go test", nil)
	if err != nil || !allow || !remember {
		t.Fatalf("approve = %v %v %v", allow, remember, err)
	}
}

func TestConnectMCPSpecNilHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c := New(Options{Sink: event.Discard, PluginCtx: ctx, Registry: tool.NewRegistry()})
	defer c.DisconnectMCPServer("nilhost")
	if _, err := c.ConnectMCPServer(config.PluginEntry{
		Name:    "nilhost",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSubmitMcpPromptPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	reg := tool.NewRegistry()
	exec := agent.New(nil, reg, agent.NewSession("sys"), agent.Options{}, event.Discard)
	c := New(Options{
		Host:      plugin.NewHost(),
		Registry:  reg,
		PluginCtx: ctx,
		Runner:    &fakeTurnRunner{},
		Executor:  exec,
		Sink:      event.Discard,
	})
	defer c.DisconnectMCPServer("mockctrl")
	if _, err := c.ConnectMCPServer(config.PluginEntry{
		Name:    "mockctrl",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env: map[string]string{
			"GO_WANT_HELPER_PROCESS": "1",
			"GO_WANT_HELPER_PROMPTS": "1",
		},
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	for _, p := range c.Host().Prompts() {
		c.Submit("/" + p.Name)
		time.Sleep(100 * time.Millisecond)
		return
	}
	t.Fatal("no prompts registered")
}
