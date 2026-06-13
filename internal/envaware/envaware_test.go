package envaware

import (
	"context"
	"encoding/json"
	"testing"

	"arcdesk/internal/tool"
)

func TestProbeAndTools(t *testing.T) {
	snap := Probe(context.Background(), t.TempDir())
	if snap.OS == "" || snap.Arch == "" {
		t.Fatalf("snap=%+v", snap)
	}
	reg := tool.NewRegistry()
	RegisterTools(reg, snap)
	RegisterRefreshTool(reg, t.TempDir())
	status, ok := reg.Get("environment_status")
	if !ok {
		t.Fatal("missing environment_status")
	}
	out, err := status.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil || out == "" {
		t.Fatalf("status: %q err=%v", out, err)
	}
	if block := ComposeBlock(snap); block == "" {
		t.Fatal("empty compose block")
	}
}
