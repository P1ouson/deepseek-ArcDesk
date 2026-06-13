package boot

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/event"
	_ "arcdesk/internal/provider/anthropic"
)

func writeMinimalBootConfig(t *testing.T, dir string, extra string) {
	body := fmt.Sprintf(`
default_model = "test-model"

[codegraph]
enabled = false

[reporag]
enabled = false

[agent]
system_prompt = "BASE"
%s

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`, extra)
	writeFile(t, dir, "arcdesk.toml", body)
}

func TestBuildWithPlannerCoordinator(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, `
planner_model = "planner-model"

[[providers]]
name = "planner-model"
kind = "openai"
base_url = "https://example.invalid"
model = "planner-x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if !strings.Contains(ctrl.History()[0].Content, "BASE") {
		t.Fatal("missing base prompt")
	}
}

func TestBuildWithAutoPlanClassifier(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, `
auto_plan = "on"
auto_plan_classifier = "classifier-model"

[[providers]]
name = "classifier-model"
kind = "openai"
base_url = "https://example.invalid"
model = "classifier-x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
}

func TestBuildWithSubagentModelMap(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, `
subagent_model = "sub-model"

[[providers]]
name = "sub-model"
kind = "openai"
base_url = "https://example.invalid"
model = "sub-x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
}

func TestBuildWithEffortOverride(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, "")
	effort := "high"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{EffortOverride: &effort})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
}

func TestBuildWithOutputStyle(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, `output_style = "concise"`)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	sys := systemMessage(ctrl.History())
	if !strings.Contains(sys, "BASE") {
		t.Fatalf("sys=%q", sys)
	}
}

func TestBuildDeferEagerMCP(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "arcdesk.toml", fmt.Sprintf(`
default_model = "test-model"

[codegraph]
enabled = false

[reporag]
enabled = false

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"

[[plugins]]
name = "eagermock"
command = %q
args = ["-test.run=TestHelperProcess", "--"]
tier = "eager"
env = { GO_WANT_HELPER_PROCESS = "1" }
`, os.Args[0]))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{DeferEagerMCP: true})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if ctrl.Host() == nil {
		t.Fatal("expected host")
	}
}

func TestBuildRequireKeyValidation(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, "")
	t.Setenv("arcdesk_TEST_KEY_UNSET", "")
	_, err := Build(context.Background(), Options{RequireKey: true})
	if err == nil {
		t.Fatal("expected require key error")
	}
}

func TestBuildUnknownModel(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, "")
	_, err := Build(context.Background(), Options{Model: "nonexistent-model"})
	if err == nil {
		t.Fatal("expected unknown model error")
	}
}

func TestBuildWithProjectHooksUntrusted(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, "")
	writeFile(t, dir, ".arcdesk/settings.json", `{
  "hooks": { "PreToolUse": [ { "match": "bash", "command": "echo hook" } ] }
}`)
	var notices []string
	sink := eventFunc(func(e event.Event) {
		if e.Text != "" {
			notices = append(notices, e.Text)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{Sink: sink})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if !strings.Contains(strings.Join(notices, "\n"), "not trusted") {
		t.Fatalf("notices=%v", notices)
	}
}

func TestRememberPermissionRulePersists(t *testing.T) {
	dir := t.TempDir()
	rememberPermissionRule(dir, "bash(go test)")
	body, err := os.ReadFile(filepath.Join(dir, "arcdesk.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "go test") {
		t.Fatalf("config = %q", body)
	}
	rememberPermissionRule(filepath.Join(dir, "missing", "nested"), "bash(x)")
}

func TestBuildAnthropicEffortOverride(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "arcdesk.toml", `
default_model = "anthropic-model"

[codegraph]
enabled = false

[reporag]
enabled = false

[agent]
system_prompt = "BASE"

[[providers]]
name = "anthropic-model"
kind = "anthropic"
base_url = "https://example.invalid"
model = "claude"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`)
	effort := "high"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{EffortOverride: &effort})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
}

func TestBuildInvalidPlannerModel(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, `planner_model = "missing-planner"`)
	_, err := Build(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), "planner_model") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildInvalidAutoPlanClassifierModel(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, `
auto_plan = "on"
auto_plan_classifier = "missing-classifier"`)
	_, err := Build(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), "auto_plan_classifier") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildWithMaxStepsAndWorkspaceRoot(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	writeMinimalBootConfig(t, dir, "")
	var stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{
		WorkspaceRoot: dir,
		MaxSteps:      3,
		Stderr:        &stderr,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
}

func TestBuildInvalidCustomProxy(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "arcdesk.toml", `
default_model = "test-model"

[network]
proxy_mode = "custom"

[codegraph]
enabled = false

[reporag]
enabled = false

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`)
	_, err := Build(context.Background(), Options{})
	if err == nil {
		t.Fatal("expected proxy validation error")
	}
}

func TestBuildCodegraphEnabledEmitsNotice(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "arcdesk.toml", `
default_model = "test-model"

[codegraph]
enabled = true
auto_install = false

[reporag]
enabled = false

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`)
	var notices []string
	sink := eventFunc(func(e event.Event) {
		if e.Text != "" {
			notices = append(notices, e.Text)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{Sink: sink})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	joined := strings.Join(notices, "\n")
	if !strings.Contains(joined, "codegraph") {
		t.Fatalf("expected codegraph notice, got %q", joined)
	}
}

func TestBuildWithTrustedHooks(t *testing.T) {
	setupBootTest(t)
	home := os.Getenv("HOME")
	dir := t.TempDir()
	t.Chdir(dir)
	trustProject(t, home, dir)
	writeMinimalBootConfig(t, dir, "")
	writeFile(t, dir, ".arcdesk/settings.json", `{
  "hooks": { "PreToolUse": [ { "match": "bash", "command": "echo hook" } ] }
}`)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
}

func TestBuildPlannerSameModelSkipsCoordinator(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, `planner_model = "test-model"`)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if strings.Contains(ctrl.Label(), "planner") {
		t.Fatalf("label = %q", ctrl.Label())
	}
}

func TestBuildWithEnabledToolsList(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "arcdesk.toml", `
default_model = "test-model"

[codegraph]
enabled = false

[reporag]
enabled = false

[tools]
enabled = ["read_file", "bogus_tool"]

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`)
	var stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{Stderr: &stderr})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
}

func TestBuildLoadsCustomSlashCommands(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, "")
	writeFile(t, dir, ".arcdesk/commands/review.md", `---
description: review code
---
Review $ARGUMENTS`)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	for _, cmd := range ctrl.Commands() {
		if cmd.Name == "review" {
			return
		}
	}
	t.Fatal("custom review command not loaded")
}

func TestBuildWithBackgroundPluginTier(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "arcdesk.toml", fmt.Sprintf(`
default_model = "test-model"

[codegraph]
enabled = false

[reporag]
enabled = false

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"

[[plugins]]
name = "bgmock"
command = %q
args = ["-test.run=TestHelperProcess", "--"]
tier = "background"
env = { GO_WANT_HELPER_PROCESS = "1" }
`, os.Args[0]))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	time.Sleep(300 * time.Millisecond)
}

func TestBuildWithCodegraphEagerWarm(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	launcher := writeCodegraphHelper(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, ".codegraph"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "arcdesk.toml", fmt.Sprintf(`
default_model = "test-model"

[codegraph]
enabled = true
path = %q
tier = "eager"

[reporag]
enabled = false

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`, launcher))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
}

func TestBuildWithConstraintOnly(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "arcdesk.toml", `
default_model = "test-model"

[codegraph]
enabled = false

[reporag]
enabled = false

[constraint]
enabled = true

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
}

func TestBuildWithCodegraphLazyTier(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	launcher := writeCodegraphHelper(t, dir)
	writeFile(t, dir, "arcdesk.toml", fmt.Sprintf(`
default_model = "test-model"

[codegraph]
enabled = true
path = %q
tier = "lazy"

[reporag]
enabled = false

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`, launcher))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
}

func TestBuildWithReporagOnly(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "arcdesk.toml", `
default_model = "test-model"

[codegraph]
enabled = false

[reporag]
enabled = true

[dependency]
enabled = false

[callgraph]
enabled = false

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
}
