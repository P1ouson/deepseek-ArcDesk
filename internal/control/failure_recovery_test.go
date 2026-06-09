package control

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"arcdesk/internal/agent"
	"arcdesk/internal/event"
	"arcdesk/internal/i18n"
	"arcdesk/internal/provider"
	"arcdesk/internal/provider/openai"
	"arcdesk/internal/tool"
)

func TestTurnDoneSurfacesAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	prov, err := openai.New(provider.Config{Name: "deepseek", BaseURL: srv.URL, Model: "deepseek-v4", APIKey: "bad"})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}

	sink, done, events := collectSink()
	exec := agent.New(prov, tool.NewRegistry(), agent.NewSession("sys"), agent.Options{}, sink)
	ctrl := New(Options{Runner: exec, Executor: exec, Sink: sink})

	ctrl.Send("hello")
	e := waitForDone(t, done)
	if e.Err == nil {
		t.Fatal("TurnDone should carry auth error")
	}
	if e.Err.Error() == "" {
		t.Fatal("empty TurnDone error")
	}

	var sawNotice bool
	for _, ev := range *events {
		if ev.Kind == event.Notice {
			sawNotice = true
		}
	}
	_ = sawNotice // auth surfaces via turn_done err, not necessarily notice
}

func TestTurnDoneSurfacesEmptyModelResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	prov, err := openai.New(provider.Config{Name: "deepseek", BaseURL: srv.URL, Model: "deepseek-v4", APIKey: "k"})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}

	sink, done, _ := collectSink()
	exec := agent.New(prov, tool.NewRegistry(), agent.NewSession("sys"), agent.Options{}, sink)
	ctrl := New(Options{Runner: exec, Executor: exec, Sink: sink})

	ctrl.Send("hello")
	e := waitForDone(t, done)
	if e.Err == nil {
		t.Fatal("TurnDone should fail on empty model response")
	}
	if e.Err.Error() != i18n.M.AgentEmptyResponse {
		t.Fatalf("TurnDone err = %q, want %q", e.Err.Error(), i18n.M.AgentEmptyResponse)
	}
}

func TestRetryAfterEmptyResponse(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		if calls == 1 {
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	prov, err := openai.New(provider.Config{Name: "deepseek", BaseURL: srv.URL, Model: "deepseek-v4", APIKey: "k"})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}

	sink, done, _ := collectSink()
	exec := agent.New(prov, tool.NewRegistry(), agent.NewSession("sys"), agent.Options{}, sink)
	ctrl := New(Options{Runner: exec, Executor: exec, Sink: sink})

	ctrl.Send("first")
	e := waitForDone(t, done)
	if e.Err == nil {
		t.Fatal("first turn should fail empty")
	}

	ctrl.Send("retry")
	e = waitForDone(t, done)
	if e.Err != nil {
		t.Fatalf("retry turn should succeed, got %v", e.Err)
	}
	if calls < 2 {
		t.Fatalf("provider should be called twice, got %d", calls)
	}
}
