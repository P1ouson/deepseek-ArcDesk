package config

import "testing"

func TestConstraintConfigShouldEnable(t *testing.T) {
	falseVal := false
	if (ConstraintConfig{Enabled: &falseVal}).ShouldEnable() {
		t.Fatal("explicit false should disable")
	}
	if !(ConstraintConfig{}).ShouldEnable() {
		t.Fatal("default should enable")
	}
}
