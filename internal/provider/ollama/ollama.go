// Package ollama implements the Ollama /api/chat provider.
// It self-registers under the "ollama" kind so users can configure local
// Ollama instances as config entries. Ollama streams newline-delimited JSON
// chunks rather than SSE, and uses /api/chat instead of /chat/completions.
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"arcdesk/internal/netclient"
	"arcdesk/internal/provider"
)

func init() {
	provider.Register("ollama", New)
}

// New builds an Ollama provider from a resolved config.
func New(cfg provider.Config) (provider.Provider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("ollama: base_url is required for provider %q", cfg.Name)
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("ollama: model is required for provider %q", cfg.Name)
	}
	name := cfg.Name
	if name == "" {
		name = "ollama"
	}
	httpClient, err := newHTTPClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("ollama: network: %w", err)
	}
	return &client{
		name:    name,
		apiKey:  cfg.APIKey,
		keyEnv:  "",
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		model:   cfg.Model,
		http:    httpClient,
	}, nil
}

func newHTTPClient(cfg provider.Config) (*http.Client, error) {
	spec, _ := cfg.Extra["proxy_spec"].(netclient.ProxySpec)
	return netclient.NewHTTPClient(spec, netclient.TransportOptions{
		DialTimeout:           30 * time.Second,
		KeepAlive:             30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 300 * time.Second,
	})
}

type client struct {
	name    string
	apiKey  string
	keyEnv  string
	baseURL string
	model   string
	http    *http.Client
}

func (c *client) Name() string { return c.name }

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func (c *client) Stream(ctx context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	if err := json.NewEncoder(buf).Encode(c.buildRequest(req)); err != nil {
		bufPool.Put(buf)
		return nil, fmt.Errorf("%s: marshal request: %w", c.name, err)
	}
	body := make([]byte, buf.Len())
	copy(body, buf.Bytes())
	bufPool.Put(buf)

	newReq := func(ctx context.Context) (*http.Request, error) {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		return httpReq, nil
	}
	resp, err := provider.SendWithRetry(ctx, c.http, c.name, c.keyEnv, newReq)
	if err != nil {
		return nil, err
	}

	out := make(chan provider.Chunk)
	go c.streamWithReconnect(ctx, resp, newReq, out)
	return out, nil
}

const maxStreamReconnects = 3

func (c *client) streamWithReconnect(ctx context.Context, resp *http.Response, newReq func(context.Context) (*http.Request, error), out chan<- provider.Chunk) {
	defer close(out)
	for attempt := 0; ; attempt++ {
		emitted, err := c.readStream(ctx, resp, out)
		if err == nil {
			return
		}
		if emitted || attempt >= maxStreamReconnects || !provider.IsConnReset(err) {
			out <- provider.Chunk{Type: provider.ChunkError, Err: err}
			return
		}
		next, rerr := provider.SendWithRetry(ctx, c.http, c.name, c.keyEnv, newReq)
		if rerr != nil {
			out <- provider.Chunk{Type: provider.ChunkError, Err: rerr}
			return
		}
		resp = next
	}
}

func (c *client) buildRequest(req provider.Request) ollamaChatRequest {
	src := provider.SanitizeToolPairing(req.Messages)
	msgs := make([]ollamaMessage, len(src))
	for i, m := range src {
		om := ollamaMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
		if m.Role == "tool" {
			om.Role = "tool"
			for _, tc := range m.ToolCalls {
				om.ToolCalls = append(om.ToolCalls, ollamaToolCall{
					Function: ollamaFunction{
						Name:      tc.Name,
						Arguments: json.RawMessage(tc.Arguments),
					},
				})
			}
		}
		for _, tc := range m.ToolCalls {
			if m.Role == "assistant" {
				om.ToolCalls = append(om.ToolCalls, ollamaToolCall{
					Function: ollamaFunction{
						Name:      tc.Name,
						Arguments: json.RawMessage(tc.Arguments),
					},
				})
			}
		}
		msgs[i] = om
	}

	var tools []ollamaToolDef
	for _, t := range req.Tools {
		tools = append(tools, ollamaToolDef{
			Type: "function",
			Function: ollamaFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	out := ollamaChatRequest{
		Model:    c.model,
		Messages: msgs,
		Tools:    tools,
		Stream:   true,
		Options: ollamaOptions{
			Temperature: req.Temperature,
		},
	}
	if req.MaxTokens > 0 {
		out.Options.NumPredict = req.MaxTokens
	}
	// For ollama, the /api/chat endpoint uses `num_ctx` for context window size.
	// We don't need to set it here since Ollama server manages it.
	return out
}

func (c *client) readStream(ctx context.Context, resp *http.Response, out chan<- provider.Chunk) (emitted bool, _ error) {
	defer resp.Body.Close()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			resp.Body.Close()
		case <-done:
		}
	}()

	acc := map[int]*provider.ToolCall{}
	started := map[int]bool{}
	var order []int

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var sr ollamaStreamResponse
		if err := json.Unmarshal([]byte(line), &sr); err != nil {
			return emitted, fmt.Errorf("%s: decode stream: %w", c.name, err)
		}
		if sr.Error != "" {
			return emitted, fmt.Errorf("%s: %s", c.name, sr.Error)
		}

		content := sr.Message.Content
		if content != "" {
			emitted = true
			out <- provider.Chunk{Type: provider.ChunkText, Text: content}
		}

		for _, tc := range sr.Message.ToolCalls {
			cur, ok := acc[tc.Index]
			if !ok {
				cur = &provider.ToolCall{}
				acc[tc.Index] = cur
				order = append(order, tc.Index)
			}
			if tc.Function.Name != "" {
				cur.Name = tc.Function.Name
			}
			cur.Arguments += string(tc.Function.Arguments)
			if !started[tc.Index] && cur.Name != "" {
				started[tc.Index] = true
				cur.ID = fmt.Sprintf("call_%d", tc.Index)
				emitted = true
				out <- provider.Chunk{Type: provider.ChunkToolCallStart, ToolCall: &provider.ToolCall{ID: cur.ID, Name: cur.Name}}
			}
		}

		if sr.Done {
			if sr.PromptEvalCount > 0 || sr.EvalCount > 0 {
				emitted = true
				out <- provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{
					PromptTokens:     sr.PromptEvalCount,
					CompletionTokens: sr.EvalCount,
					TotalTokens:      sr.PromptEvalCount + sr.EvalCount,
				}}
			}
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return emitted, fmt.Errorf("%s: read stream: %w", c.name, err)
	}

	sort.Ints(order)
	for _, idx := range order {
		tc := acc[idx]
		if tc.ID == "" {
			tc.ID = fmt.Sprintf("call_%d", idx)
		}
		out <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: tc}
	}
	out <- provider.Chunk{Type: provider.ChunkDone}
	return emitted, nil
}

// --- Ollama wire protocol ---

type ollamaChatRequest struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Tools    []ollamaToolDef  `json:"tools,omitempty"`
	Stream   bool             `json:"stream"`
	Options  ollamaOptions    `json:"options,omitempty"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaFunction `json:"function"`
}

type ollamaFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ollamaToolDef struct {
	Type     string              `json:"type"`
	Function ollamaFunctionDef   `json:"function"`
}

type ollamaFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ollamaStreamResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role      string           `json:"role"`
		Content   string           `json:"content"`
		ToolCalls []ollamaStreamToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	Done             bool   `json:"done"`
	TotalDuration    int64  `json:"total_duration"`
	LoadDuration     int64  `json:"load_duration"`
	PromptEvalCount  int    `json:"prompt_eval_count"`
	PromptEvalDuration int64 `json:"prompt_eval_duration"`
	EvalCount        int    `json:"eval_count"`
	EvalDuration     int64  `json:"eval_duration"`
	Error            string `json:"error,omitempty"`
}

type ollamaStreamToolCall struct {
	Index    int             `json:"index"`
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}
