package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func (a *App) chatCompletionHTTP(tabID, systemPrompt, userPrompt string, maxTokens int, temperature float64) (string, error) {
	entry, err := a.currentProviderEntryForTab(tabID)
	if err != nil {
		return "", err
	}
	apiKey := strings.TrimSpace(os.Getenv(entry.APIKeyEnv))
	if apiKey == "" {
		return "", fmt.Errorf("API key not configured")
	}
	model := strings.TrimSpace(entry.Model)
	if model == "" {
		if models := entry.ModelList(); len(models) > 0 {
			model = models[0]
		}
	}
	if model == "" {
		return "", fmt.Errorf("no model configured")
	}
	base := normalizeDeepSeekBaseURL(entry.BaseURL)
	endpoint := strings.TrimRight(base, "/") + "/chat/completions"

	messages := []map[string]string{}
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, map[string]string{"role": "system", "content": systemPrompt})
	}
	messages = append(messages, map[string]string{"role": "user", "content": userPrompt})

	payload, err := json.Marshal(map[string]any{
		"model":       model,
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("chat completion failed: %s", strings.TrimSpace(string(body)))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("empty completion")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
