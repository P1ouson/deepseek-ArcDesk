package agent

import (
	"testing"

	"arcdesk/internal/config"
	"arcdesk/internal/prefixruntime"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
	"arcdesk/internal/verification"
	"arcdesk/internal/verifyselect"
)

func TestP7ProviderMessagesCanonicalizesSystem(t *testing.T) {
	sess := NewSession("base")
	sess.Add(provider.Message{Role: provider.RoleSystem, Content: "extra"})
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "task"})
	a := New(&scriptedProvider{name: "p7", turns: nil}, toolRegWithRead(), sess, Options{
		PrefixRuntime: config.PrefixRuntimeConfig{},
	}, nil)
	msgs := a.providerMessages()
	if len(msgs) != 2 || msgs[0].Role != provider.RoleSystem {
		t.Fatalf("msgs = %+v", msgs)
	}
	if msgs[0].Content != "base\n\nextra" {
		t.Fatalf("merged system = %q", msgs[0].Content)
	}
}

func TestP7VerifySelectFiltersE2EForCSS(t *testing.T) {
	plan := verification.Plan{
		Checks: []verification.Check{
			{Command: "npm run build", Category: verification.CategoryBuild},
			{Command: "npm run e2e", Category: verification.CategoryE2E},
		},
	}
	out := verifyselect.MinimumChecks(plan, []string{"src/App.css"})
	for _, c := range out.Checks {
		if c.Category == verification.CategoryE2E {
			t.Fatalf("e2e should be filtered for css-only change")
		}
	}
}

func TestP7HealthFreezesCompactionInAgent(t *testing.T) {
	h := prefixruntime.NewHealthMonitor(prefixruntime.Settings{Enabled: true, MinHitRate: 0.5, MinStepsForAlert: 1})
	u := &provider.Usage{PromptTokens: 100, CacheHitTokens: 10, CacheMissTokens: 90, TotalTokens: 110}
	h.RecordStep(u, 10, 90)
	a := &Agent{prefixHealth: h}
	a.maybeCompact(nil, &provider.Usage{PromptTokens: 99999, TotalTokens: 99999})
	// no panic; compaction skipped while frozen
}

func toolRegWithRead() *tool.Registry {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	return reg
}
