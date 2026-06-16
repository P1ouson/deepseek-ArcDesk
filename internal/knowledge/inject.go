package knowledge

import (
	"fmt"
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
	searchQuery := query
	if stderr := strings.TrimSpace(p.Stderr); stderr != "" && stderr != query {
		if searchQuery == "" {
			searchQuery = stderr
		} else {
			searchQuery = searchQuery + " " + stderr
		}
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 1
	}
	ctx := failuremem.NewSearchContext(store.WorkspaceRoot(), cfg.ResolvedTTLDays(), cfg.ShouldRequireMatchingHead())
	matches, err := store.RankedSearchSmart(ctx, searchQuery, p.Paths, limit, cfg.SemanticSettings())
	if err != nil || len(matches) == 0 {
		return ""
	}
	match := matches[0]
	entry := match.Entry
	status := entry.ProvenanceStatus(ctx)
	max := cfg.ResolvedMaxRetryHintChars()
	line := entry.SummaryLine(max - len(tagOpen) - len(tagClose) - 64)
	if match.Kind == failuremem.MatchSemantic {
		if sk := failuremem.FixSkeleton(entry.Fix); sk != "" {
			line = sk
		}
	}
	if line == "" {
		return ""
	}
	_ = store.Touch(failuremem.Fingerprint(entry))
	note := hintProvenanceNote(status)
	if match.Kind == failuremem.MatchSemantic {
		note = joinNote(note, fmt.Sprintf("match=semantic similarity=%.2f", match.Score))
	}
	return FormatHintWithMeta(line, max, entry.Confidence, status.HintHead, note)
}

func joinNote(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " " + b
}

func hintProvenanceNote(st failuremem.ProvenanceStatus) string {
	switch st.Reason {
	case "commit_mismatch":
		return "note=recorded-at-old-commit"
	default:
		return ""
	}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
