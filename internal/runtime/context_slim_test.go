package runtime

import (
	"strings"
	"testing"
)

func TestBuildVerifyContextSlimStderrOnly(t *testing.T) {
	hub := NewHub(DefaultLimits())
	hub.Ingest(KindConsole, LevelError, "console", "React duplicate key warning", nil)
	hub.Ingest(KindWails, LevelError, "wails", "runtime noise", nil)
	got := BuildVerifyContextSlim(hub, "go test ./...", "FAIL pkg\nexpected 3 got 2", true, 512)
	if strings.Contains(got, "Console") || strings.Contains(got, "Wails") {
		t.Fatalf("slim should omit hub tails: %q", got)
	}
	if !strings.Contains(got, "FAIL pkg") {
		t.Fatalf("missing stderr: %q", got)
	}
}
