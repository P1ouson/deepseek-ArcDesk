package costrouter

import "testing"

func TestClassifyTiers(t *testing.T) {
	r := New(Config{Enabled: true, ExploreModel: "flash", CompactModel: "flash"})
	if tier := r.Classify("explore the codebase for auth"); tier != TierExplore {
		t.Fatalf("tier = %q", tier)
	}
	if tier := r.Classify("summarize the session"); tier != TierCompact {
		t.Fatalf("tier = %q", tier)
	}
	if tier := r.Classify("run grep on handlers"); tier != TierExecute {
		t.Fatalf("grep alone should execute, got %q", tier)
	}
	if tier, model := r.ResolveModel("explore the codebase", "default"); tier != TierExplore || model != "flash" {
		t.Fatalf("resolve = %q %q", tier, model)
	}
}
