package config

import "testing"

func TestFilterModelsForProviderDeepSeekExcludesMimo(t *testing.T) {
	p := &ProviderEntry{
		Name:      "deepseek-flash",
		BaseURL:   "https://api.deepseek.com",
		APIKeyEnv: "DEEPSEEK_API_KEY",
		Model:     "deepseek-v4-flash",
	}
	raw := []string{"deepseek-v4-flash", "deepseek-v4-pro", "mimo-v2.5-pro"}
	got := FilterModelsForProvider(p, raw)
	if len(got) != 2 {
		t.Fatalf("filtered = %v, want 2 deepseek models", got)
	}
	for _, m := range got {
		if !stringsHasPrefix(m, "deepseek") {
			t.Errorf("unexpected model %q", m)
		}
	}
}

func TestFilterModelsForProviderMimoExcludesDeepSeek(t *testing.T) {
	p := &ProviderEntry{
		Name:      "mimo-pro",
		BaseURL:   "https://token-plan-cn.xiaomimimo.com/v1",
		APIKeyEnv: "MIMO_API_KEY",
		Model:     "mimo-v2.5-pro",
	}
	raw := []string{"deepseek-v4-flash", "mimo-v2.5-pro", "mimo-v2.5"}
	got := FilterModelsForProvider(p, raw)
	if len(got) != 2 {
		t.Fatalf("filtered = %v, want 2 mimo models", got)
	}
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
