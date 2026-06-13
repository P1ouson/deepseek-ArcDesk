package runtime

import (
	"strings"
)

// FindQuery filters stored observations by keyword and optional kind/level.
type FindQuery struct {
	Query      string
	Kind       Kind
	Level      Level
	Limit      int
	ErrorsOnly bool
}

// Find returns entries whose message, source, or meta values contain Query
// (case-insensitive), newest first.
func (h *Hub) Find(q FindQuery) []Entry {
	if h == nil {
		return nil
	}
	needle := strings.ToLower(strings.TrimSpace(q.Query))
	if needle == "" {
		return nil
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]Entry, 0, limit)
	for i := len(h.entries) - 1; i >= 0 && len(out) < limit; i-- {
		e := h.entries[i]
		if q.Kind != "" && e.Kind != q.Kind {
			continue
		}
		if q.Level != "" && e.Level != q.Level {
			continue
		}
		if q.ErrorsOnly && e.Level != LevelError && e.Level != LevelWarn {
			continue
		}
		if !entryMatches(e, needle) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func entryMatches(e Entry, needle string) bool {
	if strings.Contains(strings.ToLower(e.Message), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Source), needle) {
		return true
	}
	for k, v := range e.Meta {
		if strings.Contains(strings.ToLower(k), needle) || strings.Contains(strings.ToLower(v), needle) {
			return true
		}
	}
	return false
}
