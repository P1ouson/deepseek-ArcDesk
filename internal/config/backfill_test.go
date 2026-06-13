package config

import "testing"

func hasModel(c *Config, model string) *ProviderEntry {
	for i := range c.Providers {
		for _, m := range c.Providers[i].ModelList() {
			if m == model {
				return &c.Providers[i]
			}
		}
	}
	return nil
}

func TestBackfillDeepSeekProRestoresPro(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: "DEEPSEEK_API_KEY"},
	}}
	backfillDeepSeekPro(c)
	pro := hasModel(c, "deepseek-v4-pro")
	if pro == nil {
		t.Fatal("deepseek-v4-pro not restored")
	}
	if pro.Price == nil || pro.Price.Output != 6 {
		t.Errorf("pro price not the preset: %+v", pro.Price)
	}
}

func TestBackfillDeepSeekProInheritsKeyEnv(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: "MY_DS_KEY"},
	}}
	backfillDeepSeekPro(c)
	if pro := hasModel(c, "deepseek-v4-pro"); pro == nil || pro.APIKeyEnv != "MY_DS_KEY" {
		t.Errorf("pro should inherit the flash key env, got %+v", pro)
	}
}

func TestBackfillDeepSeekProNoopWhenProPresent(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "deepseek-flash", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash"},
		{Name: "deepseek-pro", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-pro"},
	}}
	backfillDeepSeekPro(c)
	if n := len(c.Providers); n != 2 {
		t.Errorf("providers grew to %d; should be a no-op when pro is present", n)
	}
}

func TestBackfillDeepSeekProSkipsCustomEndpoint(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "myproxy", BaseURL: "https://proxy.example.com/v1", Model: "deepseek-v4-flash"},
	}}
	backfillDeepSeekPro(c)
	if hasModel(c, "deepseek-v4-pro") != nil {
		t.Error("must not add pro for a non-official endpoint that may not serve it")
	}
}

func TestBackfillDeepSeekProSkipsNonDeepSeek(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "mimo-flash", BaseURL: "https://token-plan-cn.xiaomimimo.com/v1", Model: "mimo-v2.5"},
	}}
	backfillDeepSeekPro(c)
	if len(c.Providers) != 1 {
		t.Error("unrelated config must be untouched")
	}
}

func TestDedupeRedundantProvidersDropsDuplicatePro(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "deepseek-flash", BaseURL: "https://api.deepseek.com", Models: []string{"deepseek-v4-flash", "deepseek-v4-pro"}, APIKeyEnv: "DEEPSEEK_API_KEY"},
		{Name: "deepseek-pro", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-pro", APIKeyEnv: "DEEPSEEK_API_KEY"},
	}}
	dedupeRedundantProviders(c)
	if len(c.Providers) != 1 {
		t.Fatalf("providers = %d, want 1 grouped deepseek provider", len(c.Providers))
	}
	if c.Providers[0].Name != "deepseek-flash" {
		t.Fatalf("kept provider = %q, want deepseek-flash", c.Providers[0].Name)
	}
}

func TestPruneUnconfiguredProvidersDropsPresetsWithoutKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "ds-key")
	t.Setenv("MIMO_API_KEY", "")
	c := Default()
	pruneUnconfiguredProviders(c)
	for _, p := range c.Providers {
		if p.Name == "mimo-pro" || p.Name == "mimo-flash" {
			t.Fatalf("unexpected unconfigured provider %q", p.Name)
		}
	}
	if hasModel(c, "deepseek-v4-flash") == nil {
		t.Fatal("configured deepseek provider should remain")
	}
}

func TestPruneUnconfiguredProvidersKeepsMimoWithKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "ds-key")
	t.Setenv("MIMO_API_KEY", "mi-key")
	c := Default()
	pruneUnconfiguredProviders(c)
	if hasModel(c, "mimo-v2.5-pro") == nil {
		t.Fatal("mimo-pro should remain when MIMO_API_KEY is set")
	}
}

func TestPruneUnconfiguredProvidersKeepsDefaultModelWithoutKey(t *testing.T) {
	c := &Config{
		DefaultModel: "x",
		Providers: []ProviderEntry{{
			Name: "x", Kind: "openai", BaseURL: "https://example.invalid",
			Model: "m", APIKeyEnv: "UNSET_TEST_KEY",
		}},
	}
	pruneUnconfiguredProviders(c)
	if len(c.Providers) != 1 || c.Providers[0].Name != "x" {
		t.Fatalf("providers = %+v, want default_model kept", c.Providers)
	}
}
