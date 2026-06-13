package boot

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"arcdesk/internal/callgraph"
	"arcdesk/internal/codegraph"
	"arcdesk/internal/event"
)

func TestBridgeImpactAdapterNilCGDirect(t *testing.T) {
	a := bridgeImpactAdapter{cg: nil}
	if a.Available() {
		t.Fatal("nil callgraph index should not be available")
	}
}

func TestRememberPermissionRuleInvalidAndSaveFail(t *testing.T) {
	dir := t.TempDir()
	rememberPermissionRule(dir, "")
	body, err := os.ReadFile(filepath.Join(dir, "arcdesk.toml"))
	if err == nil && strings.Contains(string(body), "allow = [\"\"]") {
		t.Fatalf("empty rule should not be saved: %q", body)
	}

	conflict := filepath.Join(dir, "arcdesk.toml")
	if err := os.Mkdir(conflict, 0o755); err != nil {
		t.Fatal(err)
	}
	rememberPermissionRule(dir, "bash(go test)")
}

func TestBuildInvalidWorkspaceConfig(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "arcdesk.toml", "{{not valid toml")
	_, err := Build(context.Background(), Options{})
	if err == nil {
		t.Fatal("expected config load error")
	}
}

func TestBuildCodegraphNotInstalledNotice(t *testing.T) {
	if _, ok := codegraph.Resolve(""); ok {
		t.Skip("codegraph runtime is installed on this machine")
	}
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
	if !strings.Contains(joined, "not installed") {
		t.Fatalf("expected not-installed notice, got %q", joined)
	}
}

func TestBuildCodegraphAutoInstallBackground(t *testing.T) {
	if _, ok := codegraph.Resolve(""); ok {
		t.Skip("codegraph runtime is installed on this machine")
	}
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "arcdesk.toml", `
default_model = "test-model"

[codegraph]
enabled = true
auto_install = true

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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{Sink: sink})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	joined := strings.Join(notices, "\n")
	if !strings.Contains(joined, "fetching code-intelligence runtime") {
		t.Fatalf("expected auto-install notice, got %q", joined)
	}
}

func TestBuildStderrOverrideLazyTier(t *testing.T) {
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
name = "lazy-mcp"
command = %q
args = ["-test.run=TestHelperProcess", "--"]
tier = "lazy"
env = { GO_WANT_HELPER_PROCESS = "1" }
`, os.Args[0]))
	var stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{Stderr: &stderr})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
}

func TestBuildWithExploreSubagentModelProvider(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "arcdesk.toml", `
default_model = "test-model"

[codegraph]
enabled = false

[reporag]
enabled = false

[agent]
system_prompt = "BASE"
subagent_models = { explore = "explore-model" }

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"

[[providers]]
name = "explore-model"
kind = "openai"
base_url = "https://example.invalid"
model = "explore-x"
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

func TestBuildInvalidNetworkProxy(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, `
[network]
proxy_mode = "custom"
proxy_url = "://bad-proxy"
`)
	_, err := Build(context.Background(), Options{})
	if err == nil {
		t.Fatal("expected proxy validation error")
	}
}

func TestBuildSystemPromptFileMissing(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, `
[agent]
system_prompt_file = "missing-prompt.md"
`)
	_, err := Build(context.Background(), Options{})
	if err == nil {
		t.Fatal("expected system prompt file error")
	}
}

func TestBuildHooksUntrustedNotice(t *testing.T) {
	setupBootTest(t)
	dir := t.TempDir()
	t.Chdir(dir)
	writeMinimalBootConfig(t, dir, "")
	writeFile(t, dir, ".arcdesk/settings.json", `{"hooks":{"UserPromptSubmit":[{"command":"echo hi"}]}}`)
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
		t.Fatalf("expected hooks trust notice, got %q", notices)
	}
}

func TestBuildWithBackgroundStderrOverride(t *testing.T) {
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
	var stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{Stderr: &stderr})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	time.Sleep(200 * time.Millisecond)
}

func TestBridgeImpactAdapterAffectedUIError(t *testing.T) {
	dir := t.TempDir()
	idx, err := callgraph.Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	adapter := newBridgeImpactAdapter(idx)
	if _, err := adapter.AffectedUI(""); err == nil {
		t.Fatal("expected error for empty method on empty project")
	}
}

func TestRememberPermissionRuleSaveSuccess(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "arcdesk.toml", `
[permissions]
allow = ["bash(existing*)"]
`)
	rememberPermissionRule(dir, "bash(go test ./...)")
	body, err := os.ReadFile(filepath.Join(dir, "arcdesk.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "go test") {
		t.Fatalf("rule not saved: %q", body)
	}
}

func TestBuildExploreSkillRunnerEndToEnd(t *testing.T) {
	setupBootTest(t)
	t.Setenv("arcdesk_TEST_KEY_UNSET", "test-key")

	var calls int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		switch n {
		case 1:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"explore","arguments":"{\"task\":\"map the repo\"}"}}]}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
		default:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"repo mapped"}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "arcdesk.toml", fmt.Sprintf(`
default_model = "test-model"

[codegraph]
enabled = false

[reporag]
enabled = true

[agent]
system_prompt = "BASE"
max_steps = 10
subagent_models = { explore = "explore-model" }

[[providers]]
name = "test-model"
kind = "openai"
base_url = %q
model = "test"
api_key_env = "arcdesk_TEST_KEY_UNSET"

[[providers]]
name = "explore-model"
kind = "openai"
base_url = %q
model = "explore"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`, srv.URL, srv.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if err := ctrl.Run(ctx, "explore the repository layout"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls < 2 {
		t.Fatalf("expected main + subagent API calls, got %d", calls)
	}
}

func TestBuildCustomSlashCommandRender(t *testing.T) {
	setupBootTest(t)
	t.Setenv("arcdesk_TEST_KEY_UNSET", "test-key")

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		if calls == 1 {
			fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"slash_command","arguments":"{\"name\":\"review\",\"arguments\":\"main.go\"}"}}]}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
		} else {
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"done"}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

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
max_steps = 5

[[providers]]
name = "test-model"
kind = "openai"
base_url = %q
model = "test"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`, srv.URL))
	writeFile(t, dir, ".arcdesk/commands/review.md", "---\ndescription: Review code\n---\nReview $ARGUMENTS in detail.")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if err := ctrl.Run(ctx, "run the review command"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calls < 2 {
		t.Fatalf("expected tool + follow-up calls, got %d", calls)
	}
}
