// fetch.go — model auto-discovery via the OpenAI-compatible GET /models API.
package config

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"arcdesk/internal/netclient"
	"arcdesk/internal/provider/openai"
)

// FetchModels queries the provider's OpenAI-compatible GET /models endpoint and
// returns the available model IDs, sorted alphabetically.
func (e *ProviderEntry) FetchModels(ctx context.Context) ([]string, error) {
	if e.BaseURL == "" {
		return nil, fmt.Errorf("fetch models: provider %q has no base_url", e.Name)
	}
	key := e.APIKey()
	if key == "" {
		return nil, fmt.Errorf("fetch models: provider %q has no API key (set %s in .env)", e.Name, e.APIKeyEnv)
	}
	url := e.ModelsURL
	if url == "" {
		url = e.BaseURL
	}
	client, err := modelsFetchHTTPClient()
	if err != nil {
		return nil, fmt.Errorf("fetch models: network: %w", err)
	}
	return openai.FetchModelsHTTP(ctx, client, url, key)
}

func modelsFetchHTTPClient() (*http.Client, error) {
	cfg, err := Load()
	if err != nil {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}
	return netclient.NewHTTPClient(cfg.NetworkProxySpec(), netclient.TransportOptions{})
}

// SwitchableModels returns model ids for UI pickers: synced API list first, else
// a one-off live GET /models when the key is set. Never falls back to bundled presets.
func (e *ProviderEntry) SwitchableModels(ctx context.Context) []string {
	if e == nil {
		return nil
	}
	if listed := e.ListedModels(); len(listed) > 0 {
		return listed
	}
	if e.Configured() {
		if models, err := e.FetchModels(ctx); err == nil && len(models) > 0 {
			return FilterModelsForProvider(e, models)
		}
	}
	return nil
}

// FilterModelsForProvider keeps only model ids that belong to the provider's
// vendor. DeepSeek's /models endpoint also lists MiMo SKUs; exclude those when
// the provider uses DEEPSEEK_API_KEY or api.deepseek.com, and vice versa.
func FilterModelsForProvider(p *ProviderEntry, models []string) []string {
	if len(models) == 0 || p == nil {
		return models
	}
	env := strings.ToUpper(strings.TrimSpace(p.APIKeyEnv))
	switch {
	case env == "MIMO_API_KEY":
		if !IsMimoOfficialBase(p.BaseURL) {
			return models
		}
		keep := func(m string) bool { return strings.HasPrefix(strings.ToLower(m), "mimo") }
		out := make([]string, 0, len(models))
		for _, m := range models {
			if keep(m) {
				out = append(out, m)
			}
		}
		if len(out) == 0 && strings.TrimSpace(p.Model) != "" {
			return []string{p.Model}
		}
		return out
	case env == "DEEPSEEK_API_KEY":
		if !IsDeepSeekOfficialBase(p.BaseURL) {
			return models
		}
		keep := func(m string) bool { return strings.HasPrefix(strings.ToLower(m), "deepseek") }
		out := make([]string, 0, len(models))
		for _, m := range models {
			if keep(m) {
				out = append(out, m)
			}
		}
		if len(out) == 0 && strings.TrimSpace(p.Model) != "" {
			return []string{p.Model}
		}
		return out
	default:
		return models
	}
}

// RefreshProviderModelsFromAPI fetches GET /models once for apiKeyEnv and writes
// the list onto every configured provider that shares that env var.
func RefreshProviderModelsFromAPI(c *Config, apiKeyEnv string) error {
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	if apiKeyEnv == "" {
		return nil
	}
	var probe *ProviderEntry
	for i := range c.Providers {
		p := &c.Providers[i]
		if p.APIKeyEnv == apiKeyEnv && p.Configured() {
			probe = p
			break
		}
	}
	if probe == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	models, err := probe.FetchModels(ctx)
	if err != nil {
		return err
	}
	if len(models) == 0 {
		return fmt.Errorf("empty model list returned")
	}
	for i := range c.Providers {
		if c.Providers[i].APIKeyEnv != apiKeyEnv {
			continue
		}
		stored := ModelsForProviderStorage(&c.Providers[i], models)
		if len(stored) == 0 {
			continue
		}
		c.Providers[i].Models = append([]string(nil), stored...)
		c.Providers[i].Model = stored[0]
		if !c.Providers[i].HasModel(c.Providers[i].Default) {
			c.Providers[i].Default = stored[0]
		}
	}
	return nil
}

// ModelsForProviderStorage keeps the full GET /models payload for relay/custom
// endpoints; official vendor hosts still apply vendor-specific filtering.
func ModelsForProviderStorage(p *ProviderEntry, models []string) []string {
	if len(models) == 0 || p == nil {
		return nil
	}
	env := strings.ToUpper(strings.TrimSpace(p.APIKeyEnv))
	switch {
	case env == "DEEPSEEK_API_KEY" && !IsDeepSeekOfficialBase(p.BaseURL):
		return append([]string(nil), models...)
	case env == "MIMO_API_KEY" && !IsMimoOfficialBase(p.BaseURL):
		return append([]string(nil), models...)
	default:
		return FilterModelsForProvider(p, models)
	}
}
