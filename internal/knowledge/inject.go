package knowledge

import (
	"strings"

	"arcdesk/internal/config"
	"arcdesk/internal/failuremem"
)

// RetryParams selects experiences for a verify-retry user message.
type RetryParams struct {
	FailedCmd string
	Stderr    string
	Paths     []string
	Limit     int
}

// RetryHint returns a formatted knowledge-hint block for verify retries.
func RetryHint(store *failuremem.Store, cfg config.KnowledgeConfig, p RetryParams) string {
	if store == nil || !cfg.ShouldEnable() || !cfg.VerifyRetryInjectEnabled() {
		return ""
	}
	query := strings.TrimSpace(p.FailedCmd)
	if query == "" {
		query = firstLine(p.Stderr)
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 1
	}
	entries, err := store.RankedSearch(query, p.Paths, limit)
	if err != nil || len(entries) == 0 {
		return ""
	}
	max := cfg.ResolvedMaxRetryHintChars()
	line := entries[0].SummaryLine(max - len(tagOpen) - len(tagClose) - 2)
	if line == "" {
		return ""
	}
	_ = store.Touch(failuremem.Fingerprint(entries[0]))
	return FormatHint(line, max)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
