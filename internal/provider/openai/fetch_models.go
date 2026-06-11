package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"arcdesk/internal/provider/apikey"
)

// Large gateways (e.g. OpenRouter) return multi-hundred-KiB /models payloads.
const maxModelsResponseBytes = 16 << 20 // 16 MiB

// FetchModels calls the OpenAI-compatible GET /models endpoint and returns the
// available model IDs, sorted alphabetically.
func FetchModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	return FetchModelsHTTP(ctx, nil, baseURL, apiKey)
}

// FetchModelsHTTP is like FetchModels but allows a custom HTTP client (proxy-aware).
func FetchModelsHTTP(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]string, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	url := modelsListURL(baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch models: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apikey.Normalize(apiKey))
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxModelsResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("fetch models: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch models: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("fetch models: decode response: %w", err)
	}

	ids := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func modelsListURL(baseURL string) string {
	url := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if !strings.HasSuffix(url, "/models") {
		url += "/models"
	}
	return url
}
