package config

import "testing"

func TestDefaultAutoPlanOff(t *testing.T) {
	if got := Default().Agent.AutoPlan; got != "off" {
		t.Fatalf("default auto_plan = %q, want off", got)
	}
}

func TestDefaultDenyBlocksRmRf(t *testing.T) {
	deny := Default().Permissions.Deny
	if len(deny) != 1 || deny[0] != "bash(rm -rf*)" {
		t.Fatalf("default permissions.deny = %v, want [bash(rm -rf*)]", deny)
	}
}
