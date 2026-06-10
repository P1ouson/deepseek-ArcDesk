package main

import (
	"os"
	"path/filepath"
	"testing"

	"arcdesk/internal/config"
)

func TestListUntrustedMCPServers(t *testing.T) {
	home := t.TempDir()
	proj := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(proj, ".mcp.json"), []byte(`{
  "mcpServers": {
    "filesystem": {"command":"npx","args":["-y","mcp-server"]}
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	a := &App{tabs: map[string]*WorkspaceTab{
		"t1": {ID: "t1", WorkspaceRoot: proj},
	}, activeTabID: "t1"}

	got := a.ListUntrustedMCPServers()
	if len(got) != 1 || got[0].Name != "filesystem" {
		t.Fatalf("got %+v, want filesystem untrusted", got)
	}
	if got[0].ConfigPath != config.MCPJSONPathForRoot(proj) {
		t.Fatalf("config path = %q", got[0].ConfigPath)
	}
}
