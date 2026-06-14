package config

import "testing"

func TestPhasePlannerConfig(t *testing.T) {
	off := false
	cfg := PhasePlannerConfig{}
	if !cfg.ShouldEnable() {
		t.Fatal("default on")
	}
	if !cfg.GatesEnforced() {
		t.Fatal("default enforce")
	}
	if (PhasePlannerConfig{Enabled: &off}).ShouldEnable() {
		t.Fatal("explicit off")
	}
	if (PhasePlannerConfig{EnforceGates: &off}).GatesEnforced() {
		t.Fatal("gates off")
	}
}

func TestFailureMemoryConfig(t *testing.T) {
	off := false
	cfg := FailureMemoryConfig{}
	if !cfg.ShouldEnable() {
		t.Fatal("default on")
	}
	if cfg.ResolvedMaxEntries() != 500 {
		t.Fatalf("max=%d", cfg.ResolvedMaxEntries())
	}
	if (FailureMemoryConfig{Enabled: &off}).ShouldEnable() {
		t.Fatal("explicit off")
	}
	if (FailureMemoryConfig{MaxEntries: 100}).ResolvedMaxEntries() != 100 {
		t.Fatal("custom max")
	}
}

func TestKnowledgeConfig(t *testing.T) {
	off := false
	cfg := KnowledgeConfig{}
	if !cfg.ShouldEnable() || !cfg.VerifyRetryInjectEnabled() || !cfg.VerifyAutoCaptureEnabled() || !cfg.SystemPromptIndexEnabled() {
		t.Fatal("defaults on")
	}
	if cfg.ResolvedMaxRetryHintChars() != 200 || cfg.ResolvedMaxRetryStderrExcerpt() != 2048 {
		t.Fatal("defaults sizes")
	}
	if (KnowledgeConfig{Enabled: &off}).ShouldEnable() {
		t.Fatal("explicit off")
	}
	if cfg.InjectOnMessageDebugOnly() {
		t.Fatal("default inject_on_message is off")
	}
}

func TestEnvAwareConfig(t *testing.T) {
	off := false
	cfg := EnvAwareConfig{}
	if !cfg.ShouldEnable() {
		t.Fatal("default on")
	}
	if !cfg.PromptFoldEnabled() {
		t.Fatal("default fold")
	}
	if (EnvAwareConfig{Enabled: &off}).ShouldEnable() {
		t.Fatal("explicit off")
	}
	if (EnvAwareConfig{FoldIntoPrompt: &off}).PromptFoldEnabled() {
		t.Fatal("fold off")
	}
}
