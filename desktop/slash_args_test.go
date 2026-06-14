package main

import (
	"strings"
	"testing"

	"arcdesk/internal/config"
)

func TestSlashArgsModelAndEffortWithoutController(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("TEST_API_KEY", "secret")
	cfg := config.Default()
	cfg.Providers = []config.ProviderEntry{{
		Name:             "deepseek",
		Kind:             "openai",
		BaseURL:          "https://api.deepseek.com",
		APIKeyEnv:        "TEST_API_KEY",
		Models:           []string{"chat", "reasoner"},
		SupportedEfforts: []string{"high", "max"},
	}}
	cfg.DefaultModel = "deepseek/chat"
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	tab := &WorkspaceTab{
		ID:            "test",
		Scope:         "global",
		WorkspaceRoot: ".",
		model:         "deepseek/chat",
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}
	app.tabs = map[string]*WorkspaceTab{"test": tab}
	app.activeTabID = "test"

	modelArgs := app.SlashArgs("/model ")
	if len(modelArgs.Items) == 0 {
		t.Fatal("expected /model arg suggestions without controller")
	}
	foundChat := false
	for _, it := range modelArgs.Items {
		if strings.Contains(it.Label, "deepseek/chat") {
			foundChat = true
		}
	}
	if !foundChat {
		t.Fatalf("/model items = %+v, want deepseek/chat", modelArgs.Items)
	}

	effortArgs := app.SlashArgs("/effort ")
	if len(effortArgs.Items) == 0 {
		t.Fatal("expected /effort arg suggestions without controller")
	}
	hasAuto := false
	for _, it := range effortArgs.Items {
		if it.Label == "auto" {
			hasAuto = true
		}
	}
	if !hasAuto {
		t.Fatalf("/effort items = %+v, want auto level", effortArgs.Items)
	}
}

func TestCommandsBuiltinArgHints(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	cmds := app.Commands()
	want := map[string]string{
		"model":  "<provider/model>",
		"effort": "auto",
		"mcp":    "add",
		"theme":  "auto",
	}
	for _, c := range cmds {
		if hint, ok := want[c.Name]; ok {
			if c.Hint == "" || !strings.Contains(c.Hint, hint) {
				t.Fatalf("command %q hint = %q, want containing %q", c.Name, c.Hint, hint)
			}
		}
	}
	for _, name := range []string{"language", "auto-plan"} {
		var found bool
		for _, c := range cmds {
			if c.Name == name && c.Hint != "" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("command %q missing from menu or has empty hint", name)
		}
	}
}
