package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestConnectProviderAPIOpenRouterStyle(t *testing.T) {
	isolateDesktopUserDirs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" && r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer sk-test") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "openai/gpt-4o"},
				{"id": "deepseek/deepseek-chat"},
			},
		})
	}))
	defer srv.Close()

	app := NewApp()
	result, err := app.ConnectProviderAPI(srv.URL+"/v1", "sk-test")
	if err != nil {
		t.Fatalf("ConnectProviderAPI: %v", err)
	}
	if result.ModelCount < 2 {
		t.Fatalf("modelCount = %d, want >= 2", result.ModelCount)
	}
	if os.Getenv("DEEPSEEK_API_KEY") != "sk-test" {
		t.Fatal("expected key in environment")
	}
}

func TestNeedsOnboardingAnyConfiguredProvider(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("MIMO_API_KEY", "")
	app := NewApp()
	if !app.NeedsOnboarding() {
		t.Fatal("expected onboarding when no keys")
	}
	t.Setenv("DEEPSEEK_API_KEY", "sk-test")
	if app.NeedsOnboarding() {
		t.Fatal("expected no onboarding when key is set")
	}
}

func TestConnectProviderAPIRequiresBaseURL(t *testing.T) {
	app := NewApp()
	_, err := app.ConnectProviderAPI("", "sk-test")
	if err == nil || !strings.Contains(err.Error(), "base URL is required") {
		t.Fatalf("err = %v", err)
	}
}
