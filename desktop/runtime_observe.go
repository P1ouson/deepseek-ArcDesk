package main

import (
	"encoding/json"
	"strings"

	"arcdesk/internal/runtime"
)

// IngestRuntime records frontend runtime observations for a workspace tab.
// payloadJSON is a JSON array of runtime.IngestItem objects.
func (a *App) IngestRuntime(tabID, payloadJSON string) error {
	hub := a.runtimeHubForTab(tabID)
	if hub == nil {
		return nil
	}
	payloadJSON = strings.TrimSpace(payloadJSON)
	if payloadJSON == "" {
		return nil
	}
	var items []runtime.IngestItem
	if err := json.Unmarshal([]byte(payloadJSON), &items); err != nil {
		return err
	}
	hub.IngestBatch(items)
	return nil
}

func (a *App) runtimeHubForTab(tabID string) *runtime.Hub {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	tab, ok := a.tabs[tabID]
	if !ok || tab == nil || tab.Ctrl == nil {
		return nil
	}
	return tab.Ctrl.RuntimeHub()
}

func (a *App) ingestTerminalOutput(sessionID string, data []byte) {
	if a == nil || len(data) == 0 {
		return
	}
	a.mu.RLock()
	tabs := make([]*WorkspaceTab, 0, len(a.tabs))
	for _, t := range a.tabs {
		if t != nil && t.Ctrl != nil && t.Ctrl.RuntimeHub() != nil {
			tabs = append(tabs, t)
		}
	}
	a.mu.RUnlock()
	text := strings.TrimSpace(string(data))
	if text == "" {
		return
	}
	meta := map[string]string{"session_id": sessionID, "stream": "terminal"}
	for _, tab := range tabs {
		if hub := tab.Ctrl.RuntimeHub(); hub != nil {
			hub.Ingest(runtime.KindGoLog, runtime.LevelInfo, "terminal", text, meta)
		}
	}
}
