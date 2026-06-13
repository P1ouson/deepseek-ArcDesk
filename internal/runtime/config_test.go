package runtime

import (
	"testing"

	"arcdesk/internal/config"
)

func TestRuntimeConfigShouldEnable(t *testing.T) {
	disabled := false
	cfg := config.RuntimeConfig{Enabled: &disabled}
	if cfg.ShouldEnable() {
		t.Fatal("expected disabled")
	}
	if !(config.RuntimeConfig{}).ShouldEnable() {
		t.Fatal("default enabled")
	}
	if got := (config.RuntimeConfig{MaxEntries: 100}).ResolvedMaxEntries(); got != 100 {
		t.Fatalf("got %d", got)
	}
}
