package boot

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/plugin"
	"arcdesk/internal/sandbox"
	"arcdesk/internal/skill"
	"arcdesk/internal/tool"
	"arcdesk/internal/tool/builtin"
)

func TestMCPTrustNoticeFormats(t *testing.T) {
	got := mcpTrustNotice(config.PluginEntry{
		Name:    "fs",
		Command: "npx",
		Args:    []string{"-y", "pkg"},
		Source:  "mcpjson",
	}, "/proj/.mcp.json")
	if !strings.Contains(got, "quarantined") || !strings.Contains(got, "fs") {
		t.Fatalf("got %q", got)
	}
	gotURL := mcpTrustNotice(config.PluginEntry{Name: "http", URL: "http://x", Source: "mcpjson"}, "mcp.json")
	if !strings.Contains(gotURL, "http://x") {
		t.Fatalf("got %q", gotURL)
	}
}

func TestMCPStartupNoticeFormats(t *testing.T) {
	if text, ok := MCPStartupNotice(nil); ok || text != "" {
		t.Fatalf("got %q ok=%v", text, ok)
	}
	text, ok := MCPStartupNotice([]plugin.Failure{
		{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"},
	})
	if !ok || !strings.Contains(text, "a, b, c") || !strings.Contains(text, "+1 more") {
		t.Fatalf("got %q ok=%v", text, ok)
	}
}

func TestLSPSpecsMergesOverrides(t *testing.T) {
	specs := LSPSpecs(config.LSPConfig{
		Servers: map[string]config.LSPServer{
			"go": {
				Command:    "custom-go",
				Args:       []string{"-mode", "stdio"},
				Env:        map[string]string{"K": "V"},
				LanguageID: "golang",
				Extensions: []string{".go"},
				InstallHint: "install go",
			},
			"emptyLang": {Command: "x"},
		},
	})
	goSpec := specs["go"]
	if goSpec.Command != "custom-go" || goSpec.LanguageID != "golang" || goSpec.InstallHint != "install go" {
		t.Fatalf("go spec = %+v", goSpec)
	}
	if specs["emptyLang"].LanguageID != "emptyLang" {
		t.Fatalf("emptyLang = %+v", specs["emptyLang"])
	}
}

func TestAddBuiltinsWorkspaceAndEnabledList(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	reg := tool.NewRegistry()
	addBuiltins(reg, nil, []string{root}, sandbox.Spec{}, builtin.SearchSpec{}, &stderr, root)
	if _, ok := reg.Get("read_file"); !ok {
		t.Fatal("expected workspace read_file")
	}

	reg2 := tool.NewRegistry()
	addBuiltins(reg2, []string{"read_file", "unknown_tool"}, []string{root}, sandbox.Spec{}, builtin.SearchSpec{}, &stderr, "")
	if _, ok := reg2.Get("read_file"); !ok {
		t.Fatal("expected read_file from enabled list")
	}
	if !strings.Contains(stderr.String(), "unknown_tool") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRememberPermissionConfigPathBranches(t *testing.T) {
	if got := rememberPermissionConfigPath("/workspace"); !strings.HasSuffix(got, "arcdesk.toml") {
		t.Fatalf("got %q", got)
	}
	if got := rememberPermissionConfigPath(""); got == "" {
		t.Fatal("expected fallback path")
	}
}

func TestSubagentModelRefBranches(t *testing.T) {
	cfg := &config.Config{
		Agent: config.AgentConfig{
			SubagentModel: "default-sub",
			SubagentModels: map[string]string{
				"review": "review-model",
			},
		},
	}
	if got := subagentModelRef(cfg, skill.Skill{Name: "review"}); got != "review-model" {
		t.Fatalf("got %q", got)
	}
	if got := subagentModelRef(cfg, skill.Skill{Name: "explore", Model: "skill-model"}); got != "skill-model" {
		t.Fatalf("got %q", got)
	}
	if got := subagentModelRef(nil, skill.Skill{Name: "x"}); got != "" {
		t.Fatalf("got %q", got)
	}
	if keys := subagentModelKeys(""); keys != nil {
		t.Fatalf("keys = %v", keys)
	}
	if keys := subagentModelKeys("security-review"); len(keys) < 2 {
		t.Fatalf("keys = %v", keys)
	}
}

func TestBuildP0StackWiresFeatures(t *testing.T) {
	isolateConfigHome(t)
	t.Setenv("arcdesk_TEST_KEY_UNSET", "test-key")
	dir := copyBootCallgraphProject(t)
	t.Chdir(dir)

	writeFile(t, dir, "arcdesk.toml", `
default_model = "test-model"

[codegraph]
enabled = false

[runtime]
enabled = true
max_entries = 256

[selfdebug]
enabled = true

[constraint]
enabled = true

[reporag]
enabled = true

[lsp]
enabled = true

[verification]
enabled = true
after_write = ["go test ./..."]
auto_discover = false
max_retries = 2
on_failure = "rollback"

[dependency]
enabled = true
auto_discover = true

[callgraph]
enabled = true
auto_discover = true

[agent]
system_prompt = "BASE P0"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "arcdesk_TEST_KEY_UNSET"
`)
	writeFile(t, dir, ".mcp.json", `{
  "mcpServers": {
    "quarantine-fs": {
      "command": "node",
      "args": ["-e", "process.exit(0)"]
    }
  }
}`)

	var notices []string
	sink := eventFunc(func(e event.Event) {
		if e.Text != "" {
			notices = append(notices, e.Text)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{Sink: sink})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	if ctrl.RuntimeHub() == nil {
		t.Fatal("expected runtime hub")
	}
	if ctrl.Skills() == nil {
		t.Fatal("expected skills slice")
	}
	joined := strings.Join(notices, "\n")
	if !strings.Contains(joined, "quarantined") {
		t.Fatalf("expected MCP trust notice, notices=%q", joined)
	}
	sys := systemMessage(ctrl.History())
	if !strings.Contains(sys, "Project repository map") {
		t.Fatalf("expected repomap in prefix: %q", sys)
	}
}

func TestAddBuiltinsWarnsUnknownTool(t *testing.T) {
	var stderr bytes.Buffer
	reg := tool.NewRegistry()
	addBuiltins(reg, []string{"read_file", "bogus_tool"}, nil, sandbox.Spec{}, builtin.SearchSpec{}, &stderr, "")
	if !strings.Contains(stderr.String(), "bogus_tool") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestAddBuiltinsAllDefaultsConfined(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	reg := tool.NewRegistry()
	addBuiltins(reg, nil, []string{root}, sandbox.Spec{}, builtin.SearchSpec{}, &stderr, "")
	if _, ok := reg.Get("read_file"); !ok {
		t.Fatal("expected read_file from all builtins path")
	}
}

func TestNewProviderInvalidKind(t *testing.T) {
	_, err := NewProvider(&config.ProviderEntry{Kind: "not-a-provider", Name: "x"})
	if err == nil {
		t.Fatal("expected provider error")
	}
}

type eventFunc func(event.Event)

func (f eventFunc) Emit(e event.Event) { f(e) }
