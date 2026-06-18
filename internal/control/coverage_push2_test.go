package control

import (
	"context"
	"fmt"
	"io"
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
	"arcdesk/internal/runtime"
	"arcdesk/internal/skill"
	"arcdesk/internal/tool"
)

func TestComposeWithPendingMemory(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	c.QueueMemory("project fact added")
	got := c.Compose("hello")
	if !strings.Contains(got, "memory-update") || !strings.Contains(got, "project fact added") {
		t.Fatalf("Compose = %q", got)
	}
}

func TestSaveDocQueuesEditNote(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "ARCDESK.md")
	if err := os.WriteFile(docPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	mem := &memory.Set{
		Docs: []memory.Source{{Scope: "project", Path: docPath, Body: "old"}},
	}
	c := New(Options{Memory: mem, Sink: event.Discard})
	if _, err := c.SaveDoc(docPath, "new body"); err != nil {
		t.Fatal(err)
	}
	if got := c.Compose("turn"); !strings.Contains(got, "new body") {
		t.Fatalf("Compose = %q", got)
	}
}

func TestAddMCPServerPersistsConfig(t *testing.T) {
	home := isolateControlHome(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("arcdesk.toml", []byte("config_version = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = home
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{
		PluginCtx: ctx,
		Registry:  tool.NewRegistry(),
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
	if names := c.ConfiguredMCPNames(); len(names) == 0 || names[0] != "persist-mcp" {
		t.Fatalf("configured = %v", names)
	}
}

func TestNewProviderAutoPlanClassifier(t *testing.T) {
	if got := NewProviderAutoPlanClassifier(nil); got != nil {
		t.Fatal("nil provider should yield nil classifier")
	}
	cl := NewProviderAutoPlanClassifier(&fakeCtrlProvider{reply: `{"needs_plan":true,"reason":"large change"}`})
	ok, reason, err := cl.NeedsPlan(context.Background(), "implement caching layer", 55)
	if err != nil || !ok || reason == "" {
		t.Fatalf("NeedsPlan = %v %q err=%v", ok, reason, err)
	}
}

func TestMcpListTextConnectedAndFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host := plugin.NewHost()
	host.RecordFailure(plugin.Spec{Name: "bad-mcp", Command: "false"}, fmt.Errorf("spawn failed"))
	reg := tool.NewRegistry()
	c := New(Options{
		Host:      host,
		Registry:  reg,
		PluginCtx: ctx,
		Sink:      event.Discard,
	})
	defer c.DisconnectMCPServer("live-mcp")
	if _, err := c.ConnectMCPServer(config.PluginEntry{
		Name:    "live-mcp",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	}); err != nil {
		t.Fatal(err)
	}
	text := c.mcpListText()
	if !strings.Contains(text, "live-mcp") || !strings.Contains(text, "bad-mcp") {
		t.Fatalf("mcp list = %q", text)
	}
}

func TestResolveRefsImageMimeAndConfinedError(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	raw := mustBase64(t, tinyPNG)
	if got := imageMime(nil, "x.jpg"); got != "image/jpeg" {
		t.Fatalf("jpg mime = %q", got)
	}
	if got := imageMime(nil, "x.webp"); got != "image/webp" {
		t.Fatalf("webp mime = %q", got)
	}
	_ = raw
	imgPath, err := SaveImageDataURL("data:image/png;base64," + tinyPNG)
	if err != nil {
		t.Fatal(err)
	}
	allowed := filepath.Join(dir, "allowed")
	if err := os.MkdirAll(allowed, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(secret, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New(Options{Sink: event.Discard, ReadRoots: []string{allowed}, WorkspaceRoot: dir})
	if _, errs := c.ResolveRefs(context.Background(), "@secret.txt"); len(errs) == 0 {
		t.Fatal("expected confinement error")
	}
	block, errs := c.ResolveRefs(context.Background(), "@"+imgPath)
	if len(errs) != 0 || !strings.Contains(block, "image") {
		t.Fatalf("block=%q errs=%v", block, errs)
	}
}

func TestForkNamedFromCheckpoint(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "second"})
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "third"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, Label: "fork", Sink: event.Discard})
	c.SetSessionPath(agent.NewSessionPath(dir, "fork"))
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	c.cpBound[0] = 1
	c.mu.Unlock()
	path, err := c.ForkNamed(0, "experiment")
	if err != nil {
		t.Fatalf("ForkNamed: %v", err)
	}
	if path == "" || c.SessionPath() != path {
		t.Fatalf("fork path = %q session = %q", path, c.SessionPath())
	}
}

func TestCompactAndNewSessionViaSubmit(t *testing.T) {
	dir := t.TempDir()
	fp := &fakeCtrlProvider{reply: "summary"}
	sess := agent.NewSession("sys")
	for i := 0; i < 6; i++ {
		sess.Add(provider.Message{Role: provider.RoleUser, Content: strings.Repeat("u", 50)})
		sess.Add(provider.Message{Role: provider.RoleAssistant, Content: strings.Repeat("a", 50)})
	}
	exec := agent.New(fp, tool.NewRegistry(), sess, agent.Options{ContextWindow: 500, CompactRatio: 0.5}, event.Discard)
	sink := &ctrlRecordSink{}
	c := New(Options{
		Runner:     exec,
		Executor:   exec,
		Sink:       sink,
		SessionDir: dir,
		Label:      "cmp",
		Hooks:      hook.NewRunner(nil, "", nil, nil),
	})
	c.SetSessionPath(agent.NewSessionPath(dir, "cmp"))
	c.Submit("/compact focus")
	time.Sleep(300 * time.Millisecond)
	c.Submit("/new")
	time.Sleep(200 * time.Millisecond)
	if !strings.Contains(sink.joined(), "compacted") {
		t.Fatalf("notices=%v", sink.texts)
	}
}

func TestSubmitRememberCommandAndUnknownMCP(t *testing.T) {
	home := isolateControlHome(t)
	dir := t.TempDir()
	t.Chdir(dir)
	mem := memory.Load(memory.Options{CWD: dir, UserDir: filepath.Join(home, ".config", "arcdesk")})
	sink := &ctrlRecordSink{}
	c := New(Options{
		Memory: mem,
		Sink:   sink,
	})
	c.Submit("/remember durable fact")
	c.Submit("/mcp__missing__prompt arg")
	time.Sleep(50 * time.Millisecond)
	joined := sink.joined()
	if !strings.Contains(joined, "remembered") {
		t.Fatalf("missing remember notice: %q", joined)
	}
	if !strings.Contains(joined, "unknown command") {
		t.Fatalf("missing unknown mcp notice: %q", joined)
	}
}

func TestDisabledSkillsAndRuntimeHub(t *testing.T) {
	isolateControlHome(t)
	hub := runtime.NewHub(runtime.ResolvedLimits(128))
	sk := skill.Skill{Name: "review", Scope: skill.ScopeBuiltin}
	c := New(Options{
		Skills:     []skill.Skill{sk},
		AllSkills:  []skill.Skill{sk},
		RuntimeHub: hub,
		Sink:       event.Discard,
	})
	t.Cleanup(func() {
		cfg := config.LoadForEdit(config.UserConfigPath())
		_ = cfg.SetSkillEnabled("review", true)
		_ = cfg.SaveTo(config.UserConfigPath())
	})
	if err := c.SetSkillEnabled("review", false); err != nil {
		t.Fatal(err)
	}
	if c.SkillEnabled("review") {
		t.Fatal("skill should be disabled")
	}
	if len(c.DisabledSkills()) != 1 {
		t.Fatalf("disabled = %v", c.DisabledSkills())
	}
	if c.RuntimeHub() != hub {
		t.Fatal("runtime hub not wired")
	}
}

func TestJobsRunningSnapshot(t *testing.T) {
	jm := jobs.NewManager(event.Discard)
	c := New(Options{Jobs: jm, Sink: event.Discard})
	jm.Start("bash", "slow", func(ctx context.Context, out io.Writer) (string, error) {
		time.Sleep(200 * time.Millisecond)
		return "ok", nil
	})
	deadline := time.Now().Add(2 * time.Second)
	for len(c.Jobs()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if len(c.Jobs()) == 0 {
		t.Fatal("expected running job view")
	}
	time.Sleep(300 * time.Millisecond)
}

func TestSlashArgItemsWithLiveController(t *testing.T) {
	data := ArgData{
		Skills:          []skill.Skill{{Name: "explore", Description: "explore"}},
		DisabledSkills:  []skill.Skill{{Name: "review", Scope: skill.ScopeBuiltin}},
		ServerNames:     []string{"fs"},
		ConfiguredMCP:   []string{"fs", "linear"},
		DisconnectedMCP: []string{"optional"},
		ModelRefs:       []string{"deepseek-flash/x", "deepseek-pro/y"},
		CurrentModel:    "deepseek-flash/x",
	}
	for _, prefix := range []string{"/auto-plan ", "/language ", "/theme ", "/hooks ", "/mcp connect "} {
		if items, _ := SlashArgItems(prefix, data); len(items) == 0 {
			t.Fatalf("no items for %q", prefix)
		}
	}
	if items, _ := SlashArgItems("/effort ", data); len(items) == 0 {
		t.Skip("effort items need resolvable current model in config")
	}
}

func TestBranchHelperLabels(t *testing.T) {
	info := agent.BranchInfo{
		BranchMeta: agent.BranchMeta{ID: "20260101-120000.000000000-test", Name: `{"msg":"success"}`},
		Path:       "/tmp/x.jsonl",
	}
	if got := branchTitle(info, 1); !strings.Contains(got, "JSON response") {
		t.Fatalf("branchTitle = %q", got)
	}
	if got, ok := structuredBranchLabel(`{"error":"boom"}`); !ok || !strings.Contains(got, "error") {
		t.Fatalf("structured = %q ok=%v", got, ok)
	}
	if got := oneLineBranch("very long branch title that should truncate", 10); !strings.HasSuffix(got, "...") {
		t.Fatalf("oneLine = %q", got)
	}
	if !numeric("120000") || numeric("12x") {
		t.Fatal("numeric mismatch")
	}
}

func TestRunShellEmptyCommandNotice(t *testing.T) {
	sink := &ctrlRecordSink{}
	c := New(Options{Sink: sink})
	c.RunShell("   ")
	time.Sleep(50 * time.Millisecond)
	if sink.joined() == "" {
		t.Fatal("expected empty shell notice")
	}
}

func TestQuickAddCompactAndCheckpoints(t *testing.T) {
	home := isolateControlHome(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("ARCDESK.md", []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}
	mem := memory.Load(memory.Options{CWD: dir, UserDir: filepath.Join(home, ".config", "arcdesk")})
	fp := &fakeCtrlProvider{reply: "summary"}
	sess := agent.NewSession("sys")
	for i := 0; i < 5; i++ {
		sess.Add(provider.Message{Role: provider.RoleUser, Content: strings.Repeat("u", 40)})
		sess.Add(provider.Message{Role: provider.RoleAssistant, Content: strings.Repeat("a", 40)})
	}
	exec := agent.New(fp, tool.NewRegistry(), sess, agent.Options{ContextWindow: 400, CompactRatio: 0.5}, event.Discard)
	c := New(Options{
		Executor:   exec,
		Runner:     exec,
		Memory:     mem,
		SessionDir: dir,
		Sink:       event.Discard,
		Hooks:      hook.NewRunner(nil, "", nil, nil),
	})
	c.SetSessionPath(agent.NewSessionPath(dir, "qa"))
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	if path, err := c.QuickAdd(memory.ScopeProject, "quick note"); err != nil || path == "" {
		t.Fatalf("QuickAdd = %q err=%v", path, err)
	}
	nilMem := New(Options{Sink: event.Discard})
	if path, err := nilMem.QuickAdd(memory.ScopeProject, "x"); err != nil || path != "" {
		t.Fatalf("nil mem QuickAdd = %q err=%v", path, err)
	}
	bare := &memory.Set{CWD: dir, UserDir: ""}
	noUser := New(Options{Memory: bare, Sink: event.Discard})
	if _, err := noUser.QuickAdd(memory.ScopeUser, "x"); err == nil {
		t.Fatal("user scope without user dir should fail")
	}
	if c.Checkpoints() == nil {
		t.Fatal("expected checkpoints slice")
	}
	if err := c.Compact(context.Background(), "tests"); err != nil {
		t.Fatal(err)
	}
	if c := New(Options{Sink: event.Discard}); c.Compact(context.Background(), "") != nil {
		t.Fatal("nil executor compact should be nil error")
	}
}

func TestNormalizeAutoPlanAllModes(t *testing.T) {
	if got := normalizeAutoPlan(""); got != "off" {
		t.Fatalf("empty = %q", got)
	}
	if got := normalizeAutoPlan("ON"); got != "on" {
		t.Fatalf("on = %q", got)
	}
	if got := normalizeAutoPlan("weird"); got != "off" {
		t.Fatalf("weird = %q", got)
	}
}

func TestAutoPlanScoreAndMaybeAutoPlan(t *testing.T) {
	c := New(Options{
		Sink:     event.Discard,
		AutoPlan: "on",
	})
	c.maybeAutoPlan(context.Background(), "implement the entire authentication system with oauth providers and migration tests")
	if !c.PlanMode() {
		t.Fatal("expected plan mode from heuristic score")
	}
	if got := autoPlanScore("implement refactor across api database and frontend with migration tests"); got < 2 {
		t.Fatalf("score = %d", got)
	}
}

func TestReadFileRefLargeAndBinary(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(big, []byte(strings.Repeat("x", maxFileRefBytes+100)), 0o644); err != nil {
		t.Fatal(err)
	}
	text, isDir, err := readFileRef(big)
	if err != nil || isDir || !strings.Contains(text, "truncated") {
		t.Fatalf("big file: dir=%v err=%v text len=%d", isDir, err, len(text))
	}
	bin := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(bin, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, 0o644); err != nil {
		t.Fatal(err)
	}
	text, isDir, err = readFileRef(bin)
	if err != nil || isDir || !strings.Contains(text, "binary file") {
		t.Fatalf("binary: dir=%v err=%v text=%q", isDir, err, text)
	}
}

func TestExplainErrorAndClampRunes(t *testing.T) {
	if got := ExplainError(nil); got != nil {
		t.Fatal("nil error")
	}
	if got := ExplainError(&provider.APIError{Status: 400, Body: `{"error":{"message":"context too long"}}`}); got == nil {
		t.Fatal("expected api error message")
	}
	if got := clampRunes("hello", 3); !strings.HasPrefix(got, "hel") {
		t.Fatalf("clamp = %q", got)
	}
}

func TestSetModeCombined(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	c.SetMode(true, true)
	if !c.PlanMode() {
		t.Fatal("plan mode")
	}
	c.SetBypass(true)
	if !c.Bypass() {
		t.Fatal("bypass")
	}
}

func TestPlanTodosJSONPaths(t *testing.T) {
	if got := PlanTodosJSON(""); got != "" {
		t.Fatalf("empty plan = %q", got)
	}
	c := New(Options{Sink: event.Discard})
	args := c.seedPlanTodos("- one\n- two")
	if args == "" {
		t.Fatal("expected seeded todo args")
	}
	c.completePlanTodos(args)
	if got := completedPlanTodosJSON("not-json"); got != "" {
		t.Fatalf("invalid json = %q", got)
	}
}
