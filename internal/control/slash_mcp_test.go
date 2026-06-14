package control

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/plugin"
	"arcdesk/internal/skill"
	"arcdesk/internal/tool"
)

func TestHandleMCPSlashSubcommands(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host := plugin.NewHost()
	host.RecordFailure(plugin.Spec{Name: "broken", Command: "false"}, errTest("spawn failed"))

	var notices []string
	c := New(Options{
		Host:      host,
		PluginCtx: ctx,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
	})
	c.handleMCPSlash("/mcp")
	if text := lastNotice(notices); !strings.Contains(text, "MCP servers") && !strings.Contains(text, "mcp") {
		t.Fatalf("/mcp list = %q", text)
	}

	notices = nil
	c.handleMCPSlash("/mcp show missing")
	if text := lastNotice(notices); !strings.Contains(text, "not found") {
		t.Fatalf("/mcp show missing = %q", text)
	}

	notices = nil
	c.handleMCPSlash("/mcp tools missing")
	if text := lastNotice(notices); !strings.Contains(text, "not found") {
		t.Fatalf("/mcp tools missing = %q", text)
	}

	notices = nil
	c.handleMCPSlash("/mcp show broken")
	if text := lastNotice(notices); !strings.Contains(text, "failed") || !strings.Contains(text, "broken") {
		t.Fatalf("/mcp show broken = %q", text)
	}

	notices = nil
	c.handleMCPSlash("/mcp unknown-sub")
	if text := lastNotice(notices); !strings.Contains(text, "unknown /mcp subcommand") {
		t.Fatalf("/mcp unknown = %q", text)
	}

	notices = nil
	c.handleMCPSlash("/mcp add")
	if text := lastNotice(notices); !strings.Contains(text, "missing server name") {
		t.Fatalf("/mcp add usage = %q", text)
	}

	notices = nil
	c.handleMCPSlash("/mcp remove")
	if text := lastNotice(notices); !strings.Contains(text, "usage: /mcp remove") {
		t.Fatalf("/mcp remove usage = %q", text)
	}
}

func TestHandleMCPSlashShowConfiguredDisconnected(t *testing.T) {
	_ = isolateControlHome(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("arcdesk.toml", []byte(`
[[plugins]]
name = "github"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	var notices []string
	c := New(Options{
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
	})
	c.handleMCPSlash("/mcp show github")
	text := lastNotice(notices)
	for _, want := range []string{"github", "configured", "not connected", "npx"} {
		if !strings.Contains(text, want) {
			t.Fatalf("/mcp show github missing %q:\n%s", want, text)
		}
	}

	notices = nil
	c.handleMCPSlash("/mcp tools github")
	text = lastNotice(notices)
	if !strings.Contains(text, "not connected") {
		t.Fatalf("/mcp tools github = %q", text)
	}

	notices = nil
	c.handleMCPSlash("/mcp")
	text = lastNotice(notices)
	if !strings.Contains(text, "github") || !strings.Contains(text, "not connected") {
		t.Fatalf("/mcp status = %q", text)
	}
}

func TestHandleMCPSlashShowConnectedTools(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host := plugin.NewHost()
	reg := tool.NewRegistry()
	c := New(Options{
		Host:      host,
		Registry:  reg,
		PluginCtx: ctx,
		Sink:      event.Discard,
	})
	defer c.DisconnectMCPServer("helper-mcp")
	if _, err := c.ConnectMCPServer(config.PluginEntry{
		Name:    "helper-mcp",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	}); err != nil {
		t.Fatalf("connect: %v", err)
	}

	show := c.mcpShowText("helper-mcp")
	if !strings.Contains(show, "connected") || !strings.Contains(show, "helper-mcp") {
		t.Fatalf("show connected = %q", show)
	}

	tools := c.mcpToolsText("helper-mcp")
	if !strings.Contains(tools, "helper-mcp tools") {
		t.Fatalf("tools header = %q", tools)
	}
}

func TestManagementNoticeMCPShowToolsRemove(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host := plugin.NewHost()
	host.RecordFailure(plugin.Spec{Name: "broken", Command: "false"}, errTest("spawn failed"))
	var notices []string
	c := New(Options{
		Host:      host,
		PluginCtx: ctx,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
	})

	if !c.managementNotice("/mcp show broken") {
		t.Fatal("expected /mcp show handled")
	}
	if text := lastNotice(notices); !strings.Contains(text, "broken") {
		t.Fatalf("show notice = %q", text)
	}

	notices = nil
	if !c.managementNotice("/mcp tools broken") {
		t.Fatal("expected /mcp tools handled")
	}
	if text := lastNotice(notices); !strings.Contains(text, "failed to connect") && !strings.Contains(text, "not found") {
		t.Fatalf("tools notice = %q", text)
	}

	notices = nil
	if !c.managementNotice("/mcp remove missing-server") {
		t.Fatal("expected /mcp remove handled")
	}
	if text := lastNotice(notices); !strings.Contains(text, "mcp remove:") {
		t.Fatalf("remove notice = %q", text)
	}
}

func TestSkillShowAndPathsText(t *testing.T) {
	skills := []skill.Skill{{
		Name:        "explore",
		Description: "Explore the codebase",
		Scope:       skill.ScopeProject,
		Path:        "/tmp/.arcdesk/skills/explore/SKILL.md",
	}}
	c := New(Options{Sink: event.Discard, Skills: skills, AllSkills: skills})
	got := c.skillShowText("explore")
	for _, want := range []string{"explore", "Explore the codebase", "enabled", "/tmp/.arcdesk/skills/explore/SKILL.md"} {
		if !strings.Contains(got, want) {
			t.Fatalf("skillShowText missing %q:\n%s", want, got)
		}
	}
	paths := c.skillPathsText()
	if !strings.Contains(paths, "skill paths:") || !strings.Contains(paths, "/tmp/.arcdesk/skills/explore/SKILL.md") {
		t.Fatalf("skillPathsText = %q", paths)
	}
}

func TestManagementNoticeSkillsShow(t *testing.T) {
	skills := []skill.Skill{{Name: "explore", Description: "Explore", Path: "/tmp/skill.md", Scope: skill.ScopeProject}}
	var notices []string
	c := New(Options{
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
		Skills:    skills,
		AllSkills: skills,
	})
	if !c.managementNotice("/skills show explore") {
		t.Fatal("expected managementNotice to handle /skills show")
	}
	if text := lastNotice(notices); !strings.Contains(text, "explore") {
		t.Fatalf("notice = %q", text)
	}
}

func TestManagementNoticeMCPSubcommandsDistinct(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host := plugin.NewHost()
	host.RecordFailure(plugin.Spec{Name: "broken", Command: "false"}, errTest("spawn failed"))
	var notices []string
	c := New(Options{
		Host:      host,
		PluginCtx: ctx,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
	})

	cases := map[string]string{
		"/mcp":              "MCP servers",
		"/mcp show broken":  "failed",
		"/mcp tools broken": "broken",
	}
	for cmd, want := range cases {
		notices = nil
		if !c.managementNotice(cmd) {
			t.Fatalf("managementNotice did not handle %q", cmd)
		}
		text := lastNotice(notices)
		if !strings.Contains(text, want) {
			t.Fatalf("%q notice = %q, want substring %q", cmd, text, want)
		}
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }

func lastNotice(notices []string) string {
	if len(notices) == 0 {
		return ""
	}
	return notices[len(notices)-1]
}

func TestManagementNoticeMCPConnectUsesTokenize(t *testing.T) {
	_ = isolateControlHome(t)
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
	if !c.managementNotice("/mcp connect cfg-mcp") {
		t.Fatal("expected connect handled")
	}
	if !strings.Contains(strings.Join(notices, "\n"), "connected") {
		t.Fatalf("notices=%v", notices)
	}
}
