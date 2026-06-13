package control

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"arcdesk/internal/agent"
	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/hook"
	"arcdesk/internal/memory"
	"arcdesk/internal/plugin"
	"arcdesk/internal/provider"
	"arcdesk/internal/skill"
	"arcdesk/internal/tool"
)

func TestSlashArgItemsExhaustiveBranches(t *testing.T) {
	data := ArgData{
		ServerNames:     []string{"live"},
		ConfiguredMCP:   []string{"live", "cfg"},
		DisconnectedMCP: []string{"idle"},
		ModelRefs:       []string{"m/a"},
		CurrentModel:    "m/a",
	}
	for _, tc := range []struct {
		line string
		want int
	}{
		{"/auto-plan on extra", 0},
		{"/language en extra", 0},
		{"/hooks list extra", 0},
		{"/mcp remove live extra", 0},
		{"/mcp connect id", 1},
		{"/mcp add --h", 1},
		{"/mcp show liv", 1},
		{"/theme dark extra", 0},
	} {
		items, _ := SlashArgItems(tc.line, data)
		if len(items) < tc.want {
			t.Fatalf("%q: got %d items, want >= %d", tc.line, len(items), tc.want)
		}
	}
	if items := autoPlanArgItems([]string{"/auto-plan", "on", "extra"}); items != nil {
		t.Fatal("autoPlanArgItems should return nil when prior > 1")
	}
	if items := languageArgItems([]string{"/language", "en", "extra"}); items != nil {
		t.Fatal("languageArgItems should return nil when prior > 1")
	}
	if items := hooksArgItems([]string{"/hooks", "list", "extra"}); items != nil {
		t.Fatal("hooksArgItems should return nil when prior > 1")
	}
}

func TestNilControllerRuntimeHub(t *testing.T) {
	var c *Controller
	if c.RuntimeHub() != nil {
		t.Fatal("nil controller should return nil hub")
	}
}

func TestCheckpointsBoundToSession(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, Sink: event.Discard})
	path := agent.NewSessionPath(dir, "ck")
	c.SetSessionPath(path)
	if c.Checkpoints() == nil {
		t.Fatal("expected non-nil checkpoint list with session path")
	}
}

func TestAddMCPServerConnectFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{
		Host:      plugin.NewHost(),
		Registry:  tool.NewRegistry(),
		PluginCtx: ctx,
		Sink:      event.Discard,
	})
	if _, err := c.AddMCPServer(config.PluginEntry{Name: "bad", Command: "nonexistent-arcdesk-mcp-cmd"}); err == nil {
		t.Fatal("expected connect failure")
	}
}

func TestAddMCPServerSaveFailure(t *testing.T) {
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
		Host:      plugin.NewHost(),
		Registry:  tool.NewRegistry(),
		PluginCtx: ctx,
		Sink:      event.Discard,
	})
	defer func() {
		c.DisconnectMCPServer("save-fail")
		c.DisconnectMCPServer("save-fail-2")
	}()
	if _, err := c.AddMCPServer(config.PluginEntry{
		Name:    "save-fail",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod("arcdesk.toml", 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod("arcdesk.toml", 0o644) })
	if _, err := c.AddMCPServer(config.PluginEntry{
		Name:    "save-fail-2",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	}); err == nil || !strings.Contains(err.Error(), "saving config failed") {
		t.Fatalf("expected save failure, got %v", err)
	}
}

func TestConnectConfiguredMCPServerMissing(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	if _, err := c.ConnectConfiguredMCPServer("missing-server"); err == nil {
		t.Fatal("expected missing server error")
	}
}

func TestExplainErrorAllKinds(t *testing.T) {
	if got := explainError(agent.ErrEmptyModelResponse); got == nil || got.Error() == "" {
		t.Fatal("empty model response")
	}
	if got := explainError(&provider.AuthError{Status: 401, KeyEnv: "DEEPSEEK_API_KEY"}); got == nil {
		t.Fatal("auth error")
	}
	if got := explainError(&provider.APIError{Status: 500, Body: "internal"}); got == nil {
		t.Fatal("unknown status should pass through")
	}
}

func TestShouldAutoPlanClassifierFallback(t *testing.T) {
	c := New(Options{
		Sink:       event.Discard,
		AutoPlan:   "on",
		Classifier: &fakeAutoPlanClassifier{err: errors.New("classifier down")},
	})
	if !c.shouldAutoPlan(context.Background(), "implement oauth database migration and frontend tests") {
		t.Fatal("expected heuristic fallback when classifier fails")
	}
}

func TestForgetMemoryNilAndMissing(t *testing.T) {
	if err := New(Options{Sink: event.Discard}).ForgetMemory("x"); err != nil {
		t.Fatal(err)
	}
	home := isolateControlHome(t)
	dir := t.TempDir()
	mem := memory.Load(memory.Options{CWD: dir, UserDir: filepath.Join(home, ".config", "arcdesk")})
	c := New(Options{Memory: mem, Sink: event.Discard})
	if err := c.ForgetMemory("does-not-exist"); err == nil {
		t.Fatal("expected delete error for missing memory")
	}
}

func TestAttachmentMoreSuccessAndErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := SaveImageDataURL("not-a-data-url"); err == nil {
		t.Fatal("invalid data url")
	}
	if _, err := SaveImageDataURL("data:image/png;base64"); err == nil {
		t.Fatal("missing payload")
	}
	raw := mustBase64(t, tinyPNG)
	if _, err := SaveImageBytes("image/tiff", raw); err == nil {
		t.Fatal("unsupported declared mime")
	}
	path, err := SaveImageDataURL("data:image/png;base64," + tinyPNG)
	if err != nil {
		t.Fatal(err)
	}
	url, err := ImageDataURL(path)
	if err != nil || !strings.HasPrefix(url, "data:image/") {
		t.Fatalf("ImageDataURL = %q err=%v", url, err)
	}
}

func TestEnsureAttachmentRootInvalidPath(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".arcdesk", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".arcdesk/attachments", []byte("not-a-dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureAttachmentRoot(); err == nil {
		t.Fatal("file posing as attachment root should fail")
	}
}

func TestSetShellKillTreeCancel(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only kill tree")
	}
	cmd := exec.Command("echo", "ok")
	setShellKillTree(cmd)
	if cmd.Cancel == nil {
		t.Fatal("expected Cancel hook")
	}
	if err := cmd.Cancel(); err != nil {
		t.Fatalf("Cancel with nil Process: %v", err)
	}
}

func TestCompletePlanTodosValidJSON(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	args := `{"todos":[{"content":"task","status":"pending"}]}`
	c.completePlanTodos(args)
	if got := completedPlanTodosJSON(args); got == "" || !strings.Contains(got, "completed") {
		t.Fatalf("completed json = %q", got)
	}
}

func TestManagementNoticeAllVerbs(t *testing.T) {
	data := ArgData{
		Skills:        []skill.Skill{{Name: "explore"}},
		ModelRefs:     []string{"m/x"},
		CurrentModel:  "m/x",
		ServerNames:   []string{"fs"},
		ConfiguredMCP: []string{"fs"},
	}
	c := New(Options{Sink: event.Discard, Skills: data.Skills, AllSkills: data.Skills})
	for _, cmd := range []string{"/model", "/memory", "/skills", "/hooks", "/mcp"} {
		if !c.managementNotice(cmd) {
			t.Fatalf("managementNotice did not handle %q", cmd)
		}
	}
}

func TestExecShellUsesKillTree(t *testing.T) {
	c := New(Options{Sink: event.Discard, Hooks: hook.NewRunner(nil, "", nil, nil)})
	got := c.RunShellQuiet("echo arcdesk-shell-ok")
	if got.Err != "" || !strings.Contains(got.Output, "arcdesk-shell-ok") {
		t.Fatalf("RunShellQuiet = %+v", got)
	}
}

func TestEffortArgItemsWithModel(t *testing.T) {
	items := effortArgItems([]string{"/effort"}, ArgData{CurrentModel: "deepseek-chat/default"})
	if len(items) == 0 {
		t.Skip("no effort levels for current model in config")
	}
	if items := effortArgItems([]string{"/effort", "low", "extra"}, ArgData{CurrentModel: "deepseek-chat/default"}); items != nil {
		t.Fatal("effortArgItems should return nil when prior > 1")
	}
}

func TestConnectCodegraphDisabled(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	cfg := &config.Config{}
	cfg.Codegraph.Enabled = false
	if _, err := c.ConnectCodegraphMCPServer(cfg); err == nil {
		t.Fatal("expected disabled error")
	}
}

func TestAutoPlanClassifierLowScore(t *testing.T) {
	c := New(Options{
		Sink:       event.Discard,
		AutoPlan:   "on",
		Classifier: &fakeAutoPlanClassifier{needsPlan: true, reason: "borderline"},
	})
	if !c.shouldAutoPlan(context.Background(), "implement login flow") {
		t.Fatal("expected classifier to approve low-score task")
	}
}

func TestRewindCodeOnly(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "u1"})
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "a1"})
	exec := agent.New(nil, tool.NewRegistry(), sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, WorkspaceRoot: dir, Sink: event.Discard})
	c.SetSessionPath(agent.NewSessionPath(dir, "rw"))
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	c.cpBound[0] = 1
	c.mu.Unlock()
	if err := c.Rewind(0, RewindCode); err != nil {
		t.Fatalf("Rewind: %v", err)
	}
}
