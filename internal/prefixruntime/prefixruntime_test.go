package prefixruntime

import (
	"testing"

	"arcdesk/internal/provider"
)

func TestForProviderRequestMergesSystem(t *testing.T) {
	in := []provider.Message{
		{Role: provider.RoleSystem, Content: "base"},
		{Role: provider.RoleUser, Content: "hi"},
		{Role: provider.RoleAssistant, Content: "hello"},
		{Role: provider.RoleSystem, Content: "extra rules"},
	}
	out := ForProviderRequest(in)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
	if out[0].Role != provider.RoleSystem || out[0].Content != "base\n\nextra rules" {
		t.Fatalf("system = %#v", out[0])
	}
	if out[1].Content != "hi" || out[2].Content != "hello" {
		t.Fatalf("tail order changed: %+v", out[1:])
	}
}

func TestForProviderRequestIdempotentSingleSystem(t *testing.T) {
	in := []provider.Message{
		{Role: provider.RoleSystem, Content: "only"},
		{Role: provider.RoleUser, Content: "task"},
	}
	out := ForProviderRequest(in)
	if len(out) != 2 || out[0].Content != "only" {
		t.Fatalf("out = %+v", out)
	}
}

func TestHealthMonitorLowHitFreezesCompaction(t *testing.T) {
	h := NewHealthMonitor(Settings{Enabled: true, MinHitRate: 0.4, MinStepsForAlert: 1})
	u := &provider.Usage{PromptTokens: 100, CacheHitTokens: 10, CacheMissTokens: 90, TotalTokens: 110}
	act := h.RecordStep(u, 10, 90)
	if !act.FreezeCompaction || act.Notice == "" {
		t.Fatalf("act = %+v", act)
	}
	if !h.FreezeCompaction() {
		t.Fatal("expected freeze latch")
	}
}

func TestRuminationSchedulerInterrupt(t *testing.T) {
	r := NewRuminationScheduler(RuminationSettings{Enabled: true, MaxReasonOnlySteps: 2, MinReasoningRunes: 10})
	if got := r.ObserveStep(nil, "1234567890", ""); got != "" {
		t.Fatalf("first = %q", got)
	}
	if got := r.ObserveStep(nil, "1234567890", ""); got == "" {
		t.Fatal("expected interrupt on second reasoning-only step")
	}
}
