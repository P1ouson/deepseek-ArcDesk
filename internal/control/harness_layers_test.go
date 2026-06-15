package control

import (
	"context"
	"os"
	"testing"

	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/harness"
	"arcdesk/internal/plugin"
	"arcdesk/internal/tool"
)

func TestHarnessMCPSyncOnConnectDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := New(Options{
		Host:      plugin.NewHost(),
		Registry:  tool.NewRegistry(),
		PluginCtx: ctx,
		Sink:      event.Discard,
		Plane:     harness.NewPlane(harness.Config{Layers: harness.LayerFlags{MCP: 0}}),
	})
	plane := c.Plane()
	if plane == nil {
		t.Fatal("expected plane")
	}
	if mcpLayerActive(plane.Status()) {
		t.Fatal("MCP layer should be inactive before connect")
	}

	if _, err := c.ConnectMCPServer(config.PluginEntry{
		Name:    "helper-mcp",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Env:     map[string]string{"GO_WANT_HELPER_PROCESS": "1"},
	}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.DisconnectMCPServer("helper-mcp")

	if !mcpLayerActive(plane.Status()) {
		t.Fatal("MCP layer should be active after connect")
	}
	if plane.Layers.MCP != 1 {
		t.Fatalf("Layers.MCP = %d, want 1", plane.Layers.MCP)
	}

	if !c.DisconnectMCPServer("helper-mcp") {
		t.Fatal("expected disconnect")
	}
	if mcpLayerActive(plane.Status()) {
		t.Fatal("MCP layer should be inactive after disconnect")
	}
}

func mcpLayerActive(st harness.Status) bool {
	for _, row := range st.Layers {
		if row.Layer == harness.LayerMCP {
			return row.Active
		}
	}
	return false
}
