package config

import "testing"

func TestNormalizeProviderBaseURLStripsEndpointPaths(t *testing.T) {
	cases := map[string]string{
		"https://apihub.agnes-ai.com/v1/chat/completions": "https://apihub.agnes-ai.com/v1",
		"https://relay.example.com/v1/models":             "https://relay.example.com/v1",
		"https://openrouter.ai/api/v1/completions":        "https://openrouter.ai/api/v1",
		"https://api.deepseek.com":                        "https://api.deepseek.com",
	}
	for in, want := range cases {
		if got := NormalizeProviderBaseURL(in); got != want {
			t.Errorf("NormalizeProviderBaseURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsDeepSeekOfficialBase(t *testing.T) {
	if !IsDeepSeekOfficialBase("") {
		t.Fatal("empty should normalize to official")
	}
	if !IsDeepSeekOfficialBase("https://api.deepseek.com") {
		t.Fatal("official host")
	}
	if IsDeepSeekOfficialBase("https://relay.example.com/v1") {
		t.Fatal("relay host should not be official")
	}
}

func TestDeepSeekBalanceURLRelay(t *testing.T) {
	if got := DeepSeekBalanceURL("https://relay.example.com/v1"); got != "" {
		t.Fatalf("relay balance = %q, want empty", got)
	}
	if got := DeepSeekBalanceURL("https://api.deepseek.com"); got != "https://api.deepseek.com/user/balance" {
		t.Fatalf("official balance = %q", got)
	}
}

func TestListedModelsSkipsPresets(t *testing.T) {
	p := &ProviderEntry{
		Model:     "deepseek-v4-flash",
		APIKeyEnv: "DEEPSEEK_API_KEY",
	}
	if got := p.ListedModels(); len(got) != 0 {
		t.Fatalf("preset-only = %v, want empty", got)
	}
	p.BaseURL = "https://relay.example.com/v1"
	p.Models = []string{"gpt-4o", "deepseek-v4-pro"}
	if got := p.ListedModels(); len(got) != 2 {
		t.Fatalf("synced = %v, want 2", got)
	}
}

func TestModelsForProviderStorageRelayRaw(t *testing.T) {
	p := &ProviderEntry{
		BaseURL:   "https://relay.example.com/v1",
		APIKeyEnv: "DEEPSEEK_API_KEY",
	}
	raw := []string{"gpt-4o", "claude-3", "deepseek-v4-pro"}
	got := ModelsForProviderStorage(p, raw)
	if len(got) != 3 {
		t.Fatalf("relay storage = %v, want all 3", got)
	}
}

func TestFilterModelsForProviderRelayKeepsAll(t *testing.T) {
	p := &ProviderEntry{
		Name:      "deepseek-flash",
		BaseURL:   "https://relay.example.com/v1",
		APIKeyEnv: "DEEPSEEK_API_KEY",
		Model:     "gpt-4o",
	}
	raw := []string{"gpt-4o", "deepseek-v4-pro", "claude-3"}
	got := FilterModelsForProvider(p, raw)
	if len(got) != 3 {
		t.Fatalf("relay filter = %v, want all models", got)
	}
}
