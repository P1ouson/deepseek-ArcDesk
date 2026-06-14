package knowledge

import (
	"fmt"
	"strings"

	"arcdesk/internal/config"
	"arcdesk/internal/failuremem"
)

// IndexBlock returns a short boot-time index of recent injectable experiences.
func IndexBlock(store *failuremem.Store, cfg config.KnowledgeConfig) string {
	if store == nil || !cfg.ShouldEnable() || !cfg.SystemPromptIndexEnabled() {
		return ""
	}
	maxLines := cfg.ResolvedMaxIndexLines()
	entries, err := store.List(maxLines * 2)
	if err != nil || len(entries) == 0 {
		return ""
	}
	var lines []string
	for i := len(entries) - 1; i >= 0 && len(lines) < maxLines; i-- {
		e := entries[i]
		failuremem.NormalizeEntry(&e)
		if !e.IsInjectable() {
			continue
		}
		line := e.SummaryLine(120)
		if line == "" {
			continue
		}
		lines = append(lines, "- "+line)
	}
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Project knowledge (recent fixes)\n")
	b.WriteString("These are past verify failures and fixes for this workspace. ")
	b.WriteString("Use failuremem_search when debugging; do not treat as ground truth without re-checking.\n")
	for _, ln := range lines {
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

// ListViewEntry is a JSON-friendly row for the desktop panel.
type ListViewEntry struct {
	ID         string   `json:"id"`
	Signature  string   `json:"signature"`
	Error      string   `json:"error,omitempty"`
	Fix        string   `json:"fix"`
	Paths      []string `json:"paths,omitempty"`
	Confidence string   `json:"confidence"`
	Hits       int      `json:"hits"`
	Kind       string   `json:"kind,omitempty"`
	Summary    string   `json:"summary"`
}

// ListView maps store entries for UI listing (newest first, one row per fingerprint).
func ListView(entries []failuremem.Entry, limit int) []ListViewEntry {
	if limit <= 0 {
		limit = 50
	}
	seen := make(map[string]bool, len(entries))
	out := make([]ListViewEntry, 0, limit)
	for i := len(entries) - 1; i >= 0 && len(out) < limit; i-- {
		e := entries[i]
		failuremem.NormalizeEntry(&e)
		fp := failuremem.Fingerprint(e)
		if fp != "" && seen[fp] {
			continue
		}
		if fp != "" {
			seen[fp] = true
		}
		out = append(out, ListViewEntry{
			ID:         e.ID,
			Signature:  e.Signature,
			Error:      truncateField(e.Error, 400),
			Fix:        e.Fix,
			Paths:      append([]string(nil), e.Paths...),
			Confidence: e.Confidence,
			Hits:       e.Hits,
			Kind:       e.Kind,
			Summary:    e.SummaryLine(160),
		})
	}
	return out
}

func truncateField(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// ConfirmEntry promotes a draft to user_confirmed when the user accepts it.
func ConfirmEntry(store *failuremem.Store, id string) error {
	if store == nil {
		return fmt.Errorf("knowledge store not configured")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id required")
	}
	entries, err := store.List(0)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.EqualFold(strings.TrimSpace(e.ID), id) {
			return store.Record(failuremem.Entry{
				Signature:  e.Signature,
				Error:      e.Error,
				Fix:        e.Fix,
				Paths:      e.Paths,
				Tags:       e.Tags,
				Kind:       e.Kind,
				Confidence: failuremem.ConfidenceUserConfirmed,
			})
		}
	}
	return fmt.Errorf("entry not found")
}
