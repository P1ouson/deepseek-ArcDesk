package config

import (
	"net/url"
	"strings"
)

// DeepSeekOfficialBase is the default DeepSeek API origin (no /v1 suffix).
const DeepSeekOfficialBase = "https://api.deepseek.com"

// openAIBaseURLSuffixes are endpoint paths users paste by mistake. Base URL should
// stop at /v1 (or the gateway root); chat/models paths are appended by the client.
var openAIBaseURLSuffixes = []string{
	"/chat/completions",
	"/completions",
	"/embeddings",
	"/models",
}

// NormalizeProviderBaseURL trims input, strips accidental endpoint paths, and maps
// empty to DeepSeekOfficialBase.
func NormalizeProviderBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DeepSeekOfficialBase
	}
	base := strings.TrimRight(raw, "/")
	for {
		lower := strings.ToLower(base)
		trimmed := false
		for _, suffix := range openAIBaseURLSuffixes {
			if strings.HasSuffix(lower, suffix) {
				base = strings.TrimRight(base[:len(base)-len(suffix)], "/")
				trimmed = true
				break
			}
		}
		if !trimmed {
			break
		}
	}
	return base
}

// IsDeepSeekOfficialBase reports whether baseURL targets DeepSeek's official API host.
func IsDeepSeekOfficialBase(raw string) bool {
	base := NormalizeProviderBaseURL(raw)
	if base == DeepSeekOfficialBase {
		return true
	}
	u, err := url.Parse(base)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Hostname(), "api.deepseek.com")
}

// IsMimoOfficialBase reports whether baseURL targets Xiaomi MiMo's official API host.
func IsMimoOfficialBase(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return strings.Contains(host, "xiaomimimo")
}

// DeepSeekBalanceURL returns the wallet balance endpoint for official DeepSeek only.
// Relay / proxy bases return "" so the UI skips balance readouts.
func DeepSeekBalanceURL(raw string) string {
	if !IsDeepSeekOfficialBase(raw) {
		return ""
	}
	return NormalizeProviderBaseURL(raw) + "/user/balance"
}

// SyncDeepSeekEndpoints updates base_url and balance_url for every DEEPSEEK_API_KEY provider.
func (c *Config) SyncDeepSeekEndpoints(base string) {
	c.SyncProvidersBaseURL("DEEPSEEK_API_KEY", base)
}

// SyncProvidersBaseURL updates base_url (and balance when applicable) for every
// provider sharing apiKeyEnv.
func (c *Config) SyncProvidersBaseURL(apiKeyEnv, base string) {
	if c == nil {
		return
	}
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	if apiKeyEnv == "" {
		return
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return
	}
	base = NormalizeProviderBaseURL(base)
	balance := ""
	if apiKeyEnv == "DEEPSEEK_API_KEY" {
		balance = DeepSeekBalanceURL(base)
	}
	for i := range c.Providers {
		if c.Providers[i].APIKeyEnv != apiKeyEnv {
			continue
		}
		c.Providers[i].BaseURL = base
		c.Providers[i].BalanceURL = balance
	}
}

// ProviderByAPIKeyEnv returns the first provider entry for an api_key_env.
func (c *Config) ProviderByAPIKeyEnv(apiKeyEnv string) (*ProviderEntry, bool) {
	if c == nil {
		return nil, false
	}
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	for i := range c.Providers {
		if c.Providers[i].APIKeyEnv == apiKeyEnv {
			return &c.Providers[i], true
		}
	}
	return nil, false
}

// ApplyDeepSeekProviderEndpoints normalizes BaseURL and BalanceURL on a single entry.
func ApplyDeepSeekProviderEndpoints(e *ProviderEntry) {
	if e == nil || e.APIKeyEnv != "DEEPSEEK_API_KEY" {
		return
	}
	e.BaseURL = NormalizeProviderBaseURL(e.BaseURL)
	e.BalanceURL = DeepSeekBalanceURL(e.BaseURL)
}

// ListedModels returns models for UI pickers: only ids previously synced from
// GET /models into Models. Bundled single-model presets are not shown.
func (e *ProviderEntry) ListedModels() []string {
	if e == nil || len(e.Models) == 0 {
		return nil
	}
	// Models was populated by ModelsForProviderStorage at sync time.
	return append([]string(nil), e.Models...)
}
