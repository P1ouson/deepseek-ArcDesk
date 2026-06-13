package config

import "testing"

func TestP2ConfigDefaults(t *testing.T) {
	off := false
	if (UIRagConfig{Enabled: &off}).ShouldEnable(true) {
		t.Fatal("ui rag disabled")
	}
	if !(TaskDAGConfig{}).ShouldEnable() {
		t.Fatal("task dag default on")
	}
	if !(CostRouterConfig{}).ShouldEnable() {
		t.Fatal("cost router default on")
	}
	if !(ContextCompressionConfig{}).ShouldEnable() {
		t.Fatal("ctx compress default on")
	}
	if (ContextCompressionConfig{}).ResolvedToolOutputMaxBytes() != 16*1024 {
		t.Fatal("enabled default bytes")
	}
}
