package boot

import (
	"testing"

	"arcdesk/internal/config"
	"arcdesk/internal/hook"
)

func TestFilterTrustedMCPPluginsBlocksRepoJSON(t *testing.T) {
	proj := t.TempDir()
	plugins := []config.PluginEntry{
		{Name: "explicit", Command: "ARCDESK-plugin-example"},
		{Name: "filesystem", Command: "npx", Args: []string{"-y", "pkg"}, Source: "mcpjson"},
	}
	trusted, blocked := filterTrustedMCPPlugins(proj, plugins, "")
	if len(blocked) != 1 || blocked[0].Name != "filesystem" {
		t.Fatalf("blocked = %+v, want filesystem", blocked)
	}
	if len(trusted) != 1 || trusted[0].Name != "explicit" {
		t.Fatalf("trusted = %+v, want explicit", trusted)
	}
}

func TestFilterTrustedMCPPluginsAllowsAfterTrust(t *testing.T) {
	home := t.TempDir()
	proj := t.TempDir()
	if err := hook.TrustMCPServer(proj, "filesystem", home); err != nil {
		t.Fatal(err)
	}
	plugins := []config.PluginEntry{{Name: "filesystem", Source: "mcpjson"}}
	trusted, blocked := filterTrustedMCPPlugins(proj, plugins, home)
	if len(blocked) != 0 || len(trusted) != 1 {
		t.Fatalf("trusted=%+v blocked=%+v", trusted, blocked)
	}
}
