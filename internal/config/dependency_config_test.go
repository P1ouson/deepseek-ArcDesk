package config

import "testing"

func TestAutoDiscoverSemantics(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name          string
		enabled       *bool
		autoDiscover  *bool
		hasGoMod      bool
		expectEnabled bool
	}{
		{name: "unset unset with go.mod", autoDiscover: nil, hasGoMod: true, expectEnabled: true},
		{name: "unset unset without go.mod", autoDiscover: nil, hasGoMod: false, expectEnabled: false},
		{name: "unset false with go.mod", autoDiscover: &falseVal, hasGoMod: true, expectEnabled: false},
		{name: "unset false without go.mod", autoDiscover: &falseVal, hasGoMod: false, expectEnabled: false},
		{name: "true true with go.mod", enabled: &trueVal, autoDiscover: &trueVal, hasGoMod: true, expectEnabled: true},
		{name: "true true without go.mod", enabled: &trueVal, autoDiscover: &trueVal, hasGoMod: false, expectEnabled: true},
		{name: "true false with go.mod", enabled: &trueVal, autoDiscover: &falseVal, hasGoMod: true, expectEnabled: true},
		{name: "true false without go.mod", enabled: &trueVal, autoDiscover: &falseVal, hasGoMod: false, expectEnabled: true},
		{name: "false with go.mod", enabled: &falseVal, autoDiscover: &trueVal, hasGoMod: true, expectEnabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DependencyConfig{
				Enabled:      tt.enabled,
				AutoDiscover: tt.autoDiscover,
			}
			got := cfg.ShouldIndex(tt.hasGoMod)
			if got != tt.expectEnabled {
				t.Fatalf("ShouldIndex(%v) = %v, want %v", tt.hasGoMod, got, tt.expectEnabled)
			}
		})
	}
}
