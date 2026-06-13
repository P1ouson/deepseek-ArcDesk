package config

import "testing"

func TestArchitectureGuardianConfig(t *testing.T) {
	t.Parallel()
	off := false
	if (ArchitectureGuardianConfig{Enabled: &off}).ShouldEnable() {
		t.Fatal("disabled config should not enable")
	}
	var empty ArchitectureGuardianConfig
	if !empty.ShouldEnable() {
		t.Fatal("default guardian should enable")
	}
}
