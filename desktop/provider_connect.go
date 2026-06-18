package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"arcdesk/internal/config"
	"arcdesk/internal/netclient"
	"arcdesk/internal/provider/apikey"
	"arcdesk/internal/provider/openai"
)

// ProviderConnectResult is returned after a successful generic API connection.
type ProviderConnectResult struct {
	BaseURL    string   `json:"baseUrl"`
	ModelCount int      `json:"modelCount"`
	Models     []string `json:"models"`
	KeyEnv     string   `json:"keyEnv"`
}

func normalizeConnectBaseURL(raw string) (string, error) {
	base := config.NormalizeProviderBaseURL(raw)
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("base URL is required")
	}
	return base, nil
}

func primaryAPIKeyEnv(cfg *config.Config) string {
	if cfg == nil {
		return onboardingKeyEnv
	}
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		if strings.TrimSpace(p.Kind) != "openai" {
			continue
		}
		if env := strings.TrimSpace(p.APIKeyEnv); env != "" {
			return env
		}
	}
	return onboardingKeyEnv
}

func validateOpenAICompatibleAPI(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]string, error) {
	base, err := normalizeConnectBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	models, err := openai.FetchModelsHTTP(ctx, client, base, apiKey)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("empty model list returned")
	}
	return models, nil
}

func (a *App) providerHTTPClient() (*http.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}
	return netclient.NewHTTPClient(cfg.NetworkProxySpec(), netclient.TransportOptions{})
}

// ConnectProviderAPI validates an OpenAI-compatible endpoint, stores the key,
// syncs base_url on matching providers, and refreshes the model list.
// No native confirm dialog — the onboarding/settings form is the confirmation.
func (a *App) ConnectProviderAPI(baseURL, apiKey string) (ProviderConnectResult, error) {
	var out ProviderConnectResult
	apiKey = apikey.Normalize(apiKey)
	if apiKey == "" {
		return out, fmt.Errorf("api key is required")
	}
	base, err := normalizeConnectBaseURL(baseURL)
	if err != nil {
		return out, err
	}

	ctx := a.bootContext()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client, err := a.providerHTTPClient()
	if err != nil {
		return out, fmt.Errorf("network: %w", err)
	}

	models, err := validateOpenAICompatibleAPI(ctx, client, base, apiKey)
	if err != nil {
		return out, userFacingErr(fmt.Errorf("validate: %w", err))
	}

	cfg, _ := config.Load()
	keyEnv := primaryAPIKeyEnv(cfg)
	if err := upsertDotEnv(keyEnv, apiKey); err != nil {
		return out, fmt.Errorf("save: %w", err)
	}

	var synced []string
	if err := a.applyConfigOnly(func(c *config.Config) error {
		c.SyncProvidersBaseURL(keyEnv, base)
		if err := config.RefreshProviderModelsFromAPI(c, keyEnv); err != nil {
			return err
		}
		probe, ok := c.ProviderByAPIKeyEnv(keyEnv)
		if !ok || len(probe.Models) == 0 {
			return fmt.Errorf("model sync failed")
		}
		synced = append([]string(nil), probe.Models...)
		ref := probe.Name + "/" + probe.DefaultModel()
		if probe.DefaultModel() != "" {
			_ = c.SetDefaultModel(ref)
		}
		return nil
	}); err != nil {
		return out, fmt.Errorf("config: %w", err)
	}
	if a.activeTab() != nil {
		if err := a.rebuild(); err != nil {
			return out, fmt.Errorf("rebuild: %w", err)
		}
	}
	if len(synced) == 0 {
		synced = models
	}

	out = ProviderConnectResult{
		BaseURL:    base,
		ModelCount: len(synced),
		Models:     synced,
		KeyEnv:     keyEnv,
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "agent:models-refreshed", map[string]any{
			"apiKeyEnv": keyEnv,
			"ok":        true,
		})
	}
	return out, nil
}
