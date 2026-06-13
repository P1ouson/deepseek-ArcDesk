package config

import "testing"

func TestCallgraphAutoDiscoverSemantics(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name          string
		enabled       *bool
		autoDiscover  *bool
		discoverable  bool
		expectEnabled bool
	}{
		{name: "unset unset wails", discoverable: true, expectEnabled: true},
		{name: "unset unset not wails", discoverable: false, expectEnabled: false},
		{name: "unset false wails", autoDiscover: &falseVal, discoverable: true, expectEnabled: false},
		{name: "true false wails", enabled: &trueVal, autoDiscover: &falseVal, discoverable: true, expectEnabled: true},
		{name: "false wails", enabled: &falseVal, discoverable: true, expectEnabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := CallgraphConfig{Enabled: tt.enabled, AutoDiscover: tt.autoDiscover}
			if got := cfg.ShouldIndex(tt.discoverable); got != tt.expectEnabled {
				t.Fatalf("ShouldIndex(%v) = %v, want %v", tt.discoverable, got, tt.expectEnabled)
			}
		})
	}
}
