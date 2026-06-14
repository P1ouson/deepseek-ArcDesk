package main

import (
	"strings"

	"arcdesk/internal/config"
	"arcdesk/internal/failuremem"
	"arcdesk/internal/knowledge"
)

// KnowledgeView is returned by Knowledge() for the desktop panel.
type KnowledgeView struct {
	Available bool                      `json:"available"`
	Entries   []knowledge.ListViewEntry `json:"entries"`
}

func (a *App) knowledgeStore() (*failuremem.Store, config.KnowledgeConfig, bool) {
	a.mu.RLock()
	tab := a.activeTabLocked()
	a.mu.RUnlock()
	if tab == nil {
		return nil, config.KnowledgeConfig{}, false
	}
	root := strings.TrimSpace(tab.WorkspaceRoot)
	if root == "" {
		return nil, config.KnowledgeConfig{}, false
	}
	cfg, err := config.LoadForRoot(root)
	if err != nil {
		return nil, config.KnowledgeConfig{}, false
	}
	if !cfg.FailureMemory.ShouldEnable() || cfg.Knowledge.Disabled() {
		return nil, cfg.Knowledge, false
	}
	store, err := failuremem.Open(root, cfg.FailureMemory.ResolvedMaxEntries())
	if err != nil {
		return nil, cfg.Knowledge, false
	}
	return store, cfg.Knowledge, true
}

// Knowledge lists workspace experience entries for the active tab.
func (a *App) Knowledge() KnowledgeView {
	view := KnowledgeView{Entries: []knowledge.ListViewEntry{}}
	store, _, ok := a.knowledgeStore()
	if !ok || store == nil {
		return view
	}
	view.Available = true
	entries, err := store.List(100)
	if err != nil {
		return view
	}
	view.Entries = knowledge.ListView(entries, 100)
	return view
}

// KnowledgeConfirm promotes a draft entry to user_confirmed.
func (a *App) KnowledgeConfirm(id string) error {
	store, _, ok := a.knowledgeStore()
	if !ok || store == nil {
		return nil
	}
	return knowledge.ConfirmEntry(store, id)
}

// KnowledgeStale marks an entry as stale so it is not injected.
func (a *App) KnowledgeStale(id string) error {
	store, _, ok := a.knowledgeStore()
	if !ok || store == nil {
		return nil
	}
	return store.MarkStale(id)
}

// KnowledgeCaptureInput is the payload for confirming a suggested capture.
type KnowledgeCaptureInput struct {
	ID          string   `json:"id"`
	Fingerprint string   `json:"fingerprint"`
	Signature   string   `json:"signature"`
	Summary     string   `json:"summary"`
	Error       string   `json:"error"`
	Fix         string   `json:"fix"`
	Paths       []string `json:"paths"`
}

// KnowledgeCaptureRecord writes a user-confirmed capture proposal as draft knowledge.
func (a *App) KnowledgeCaptureRecord(in KnowledgeCaptureInput) error {
	store, _, ok := a.knowledgeStore()
	if !ok || store == nil {
		return nil
	}
	return knowledge.RecordCaptureProposal(store, knowledge.CaptureProposal{
		ID:          in.ID,
		Fingerprint: in.Fingerprint,
		Signature:   in.Signature,
		Summary:     in.Summary,
		Error:       in.Error,
		Fix:         in.Fix,
		Paths:       append([]string(nil), in.Paths...),
	})
}

// KnowledgeCaptureDismiss permanently ignores a capture fingerprint for this workspace.
func (a *App) KnowledgeCaptureDismiss(fingerprint string) error {
	store, _, ok := a.knowledgeStore()
	if !ok || store == nil {
		return nil
	}
	return knowledge.DismissCapture(store.WorkspaceRoot(), fingerprint)
}
