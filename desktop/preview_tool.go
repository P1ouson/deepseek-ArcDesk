package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type webPreviewOpener interface {
	OpenWebPreview(raw string)
}

type openWebPreviewTool struct {
	opener webPreviewOpener
}

func (openWebPreviewTool) Name() string { return "open_web_preview" }

func (openWebPreviewTool) Description() string {
	return "Open the in-app Web preview side panel for a local dev server. Omit url to auto-detect common ports (5173, 3000, etc.). Use after starting a dev server or when the user asks to preview the app."
}

func (openWebPreviewTool) Schema() json.RawMessage {
	return json.RawMessage(`{
"type":"object",
"properties":{
  "url":{"type":"string","description":"Optional preview URL (e.g. http://localhost:5173). When omitted, scans common local dev ports."}
}
}`)
}

func (openWebPreviewTool) ReadOnly() bool { return true }

func (t openWebPreviewTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var p struct {
		URL string `json:"url"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &p); err != nil {
			return "", fmt.Errorf("invalid args: %w", err)
		}
	}
	if t.opener == nil {
		return "", fmt.Errorf("web preview is unavailable")
	}
	t.opener.OpenWebPreview(strings.TrimSpace(p.URL))
	if strings.TrimSpace(p.URL) != "" {
		return fmt.Sprintf("Opened Web preview at %s.", strings.TrimSpace(p.URL)), nil
	}
	return "Opened Web preview (auto-detected local dev server when available).", nil
}
