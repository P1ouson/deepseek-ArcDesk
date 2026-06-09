package agent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"arcdesk/internal/event"
	"arcdesk/internal/provider"
	"arcdesk/internal/provider/openai"
	"arcdesk/internal/tool"
)

func TestRunRejectsEmptyModelResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	prov, err := openai.New(provider.Config{Name: "deepseek", BaseURL: srv.URL, Model: "deepseek-v4", APIKey: "k"})
	if err != nil {
		t.Fatalf("New provider: %v", err)
	}

	sink := &recordSink{}
	a := New(prov, tool.NewRegistry(), NewSession(""), Options{}, sink)
	err = a.Run(context.Background(), "hi")
	if err == nil {
		t.Fatal("Run should fail on empty provider response")
	}
	if !strings.Contains(err.Error(), "empty model response") {
		t.Fatalf("err = %v, want ErrEmptyModelResponse", err)
	}

	turnDones := sink.kinds(event.TurnDone)
	if len(turnDones) != 0 {
		t.Fatalf("agent Run does not emit TurnDone; controller wraps this")
	}
}

func TestRunAcceptsToolOnlyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		body := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"glob","arguments":"{\"pattern\":\"*\"}"}}]}}]}` + "\n\n" +
			`data: [DONE]` + "\n\n"
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	prov, err := openai.New(provider.Config{Name: "deepseek", BaseURL: srv.URL, Model: "deepseek-v4", APIKey: "k"})
	if err != nil {
		t.Fatalf("New provider: %v", err)
	}

	sink := &recordSink{}
	reg := tool.NewRegistry()
	a := New(prov, reg, NewSession(""), Options{}, sink)
	// glob with no matches still counts as a tool turn, not an empty response.
	err = a.Run(context.Background(), "find files")
	if err != nil && strings.Contains(err.Error(), "empty model response") {
		t.Fatalf("tool-only stream should not be treated as empty: %v", err)
	}
}
