package prompt

import (
	"strings"
	"testing"

	"arcdesk/internal/tool"
	"arcdesk/internal/tool/builtin"
)

func TestAlignWithRegistryDesktopWorkspaceScope(t *testing.T) {
	reg := tool.NewRegistry()
	ws := builtin.Workspace{Dir: t.TempDir()}
	for _, t := range ws.Tools() {
		reg.Add(t)
	}

	got := AlignWithRegistry("You are ArcDesk.", reg)
	for _, want := range []string{
		"todo_write and complete_step are not registered",
		"Background job helpers",
		"read_file, grep, glob, and ls",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("aligned prompt missing %q:\n%s", want, got)
		}
	}
}

func TestAlignWithRegistryFullBuiltinSetUnchanged(t *testing.T) {
	reg := tool.NewRegistry()
	for _, t := range tool.Builtins() {
		reg.Add(t)
	}
	base := "You are ArcDesk."
	if got := AlignWithRegistry(base, reg); got != base {
		t.Fatalf("full registry should not append scope note:\n%s", got)
	}
}
