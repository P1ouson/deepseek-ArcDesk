package costrouter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestRouterDisabledAndFallbacks(t *testing.T) {
	var r *Router
	tier, model := r.ResolveModel("hello", "default-model")
	if tier != TierExecute || model != "default-model" {
		t.Fatalf("nil router: %q %q", tier, model)
	}
	if r.Snapshot().Enabled {
		t.Fatal("nil snapshot")
	}
	dis := New(Config{Enabled: false, DefaultModel: "main"})
	if dis.Enabled() || dis.ModelForTier(TierExplore) != "" {
		t.Fatal("disabled router")
	}
	tier, model = dis.ResolveModel("explore the codebase", "main")
	if tier != TierExplore || model != "main" {
		t.Fatalf("disabled resolve: %q %q", tier, model)
	}
}

func TestRouterAllTiers(t *testing.T) {
	r := New(Config{
		Enabled:       true,
		DefaultModel:  "main",
		ClassifyModel: "c",
		ExecuteModel:  "e",
		CompactModel:  "k",
		ExploreModel:  "x",
	})
	if r.ModelForTier(TierClassify) != "c" || r.ModelForTier(TierExecute) != "e" {
		t.Fatal("tier models")
	}
	if r.ModelForTier(Tier("unknown")) != "" {
		t.Fatal("unknown tier")
	}
	if r.Classify("") != TierExecute {
		t.Fatal("empty classify")
	}
	if r.Classify("how does this work?") != TierClassify {
		t.Fatal("question classify")
	}
	if r.Classify("implement the feature end to end with tests and docs") != TierExecute {
		t.Fatal("execute classify")
	}
	snap := r.Snapshot()
	if !snap.Enabled || snap.Tiers[TierCompact] != "k" {
		t.Fatalf("snap=%+v", snap)
	}
	tier, model := r.ResolveModel("do work", "main")
	if tier != TierExecute || model != "e" {
		t.Fatalf("execute model: %q %q", tier, model)
	}
	tier, model = r.ResolveModel("implement something long enough to stay on execute tier", "main")
	if tier != TierExecute || model != "e" {
		t.Fatalf("execute tier uses execute model: %q %q", tier, model)
	}
	fallback := New(Config{Enabled: true, DefaultModel: "main"})
	tier, model = fallback.ResolveModel("implement something long enough to stay on execute tier", "main")
	if tier != TierExecute || model != "main" {
		t.Fatalf("default fallback: %q %q", tier, model)
	}
}

func TestCostRouterToolsExecute(t *testing.T) {
	r := New(Config{Enabled: true, DefaultModel: "main", ExploreModel: "flash"})
	reg := tool.NewRegistry()
	RegisterTools(reg, r)
	status, _ := reg.Get("cost_router_status")
	out, err := status.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "Cost router") {
		t.Fatalf("status=%q err=%v", out, err)
	}
	classify, _ := reg.Get("cost_router_classify")
	out, err = classify.Execute(context.Background(), json.RawMessage(`{"prompt":"explore the codebase","default_model":"main"}`))
	if err != nil || !strings.Contains(out, "explore") || !strings.Contains(out, "flash") {
		t.Fatalf("classify=%q err=%v", out, err)
	}
	if _, err = classify.Execute(context.Background(), json.RawMessage(`{`)); err == nil {
		t.Fatal("classify invalid json")
	}
}

func TestCostRouterClassifyCompact(t *testing.T) {
	r := New(Config{Enabled: true, CompactModel: "cheap"})
	if r.Classify("please summarize the log") != TierCompact {
		t.Fatal("compact tier")
	}
	if r.Classify("look for usages") != TierExplore {
		t.Fatal("look for tier")
	}
}

func TestResolveModelClassifyTier(t *testing.T) {
	r := New(Config{Enabled: true, ClassifyModel: "cheap", DefaultModel: "main"})
	tier, model := r.ResolveModel("how does this work?", "main")
	if tier != TierClassify || model != "cheap" {
		t.Fatalf("classify route: %q %q", tier, model)
	}
}

func TestCostRouterToolMetadata(t *testing.T) {
	r := New(Config{Enabled: true})
	reg := tool.NewRegistry()
	RegisterTools(reg, r)
	for _, name := range []string{"cost_router_status", "cost_router_classify"} {
		tl, ok := reg.Get(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if tl.Description() == "" || !tl.ReadOnly() {
			t.Fatalf("metadata %s", name)
		}
		if len(tl.Schema()) == 0 {
			t.Fatalf("schema %s", name)
		}
	}
}

func TestResolveModelDefaultModelFallback(t *testing.T) {
	r := New(Config{Enabled: true, DefaultModel: "main"})
	tier, model := r.ResolveModel("how does this work?", "session")
	if tier != TierClassify || model != "main" {
		t.Fatalf("classify default: %q %q", tier, model)
	}
}

func TestCostRouterRegisterNilSafe(t *testing.T) {
	RegisterTools(nil, New(Config{}))
	RegisterTools(tool.NewRegistry(), nil)
}
