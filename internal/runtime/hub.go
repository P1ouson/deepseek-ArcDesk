package runtime

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// Hub is a thread-safe session-scoped observation store.
type Hub struct {
	mu      sync.RWMutex
	limits  Limits
	entries []Entry
	nextID  int64
	turn    int
	state   map[string]string
	byKind  map[Kind]int
	errors  int
	lastAt  time.Time
}

// NewHub returns an empty hub with the given retention limits.
func NewHub(limits Limits) *Hub {
	if limits.MaxEntries <= 0 {
		limits = DefaultLimits()
	}
	return &Hub{
		limits: limits,
		state:  make(map[string]string),
		byKind: make(map[Kind]int),
	}
}

// SetTurn records the active agent turn for subsequent entries.
func (h *Hub) SetTurn(turn int) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.turn = turn
	h.mu.Unlock()
}

// Turn returns the last recorded agent turn.
func (h *Hub) Turn() int {
	if h == nil {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.turn
}

// Ingest appends one observation.
func (h *Hub) Ingest(kind Kind, level Level, source, message string, meta map[string]string) {
	if h == nil {
		return
	}
	kind = normalizeKind(kind, source, message, meta)
	message = strings.TrimSpace(message)
	if message == "" && len(meta) == 0 {
		return
	}
	if level == "" {
		level = LevelInfo
	}
	entry := Entry{
		Kind:    kind,
		Level:   level,
		At:      time.Now().UTC(),
		Source:  strings.TrimSpace(source),
		Message: truncate(message, 8000),
		Turn:    h.Turn(),
	}
	if len(meta) > 0 {
		entry.Meta = cloneMeta(meta)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.nextID++
	entry.ID = h.nextID
	entry.Turn = h.turn
	h.entries = append(h.entries, entry)
	h.byKind[kind]++
	if level == LevelError || level == LevelWarn {
		if level == LevelError {
			h.errors++
		}
	}
	h.lastAt = entry.At
	if kind == KindState {
		h.mergeStateLocked(meta)
	}
	h.trimLocked()
}

// SetState stores a runtime state key/value (latest wins).
func (h *Hub) SetState(key, value string) {
	if h == nil {
		return
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state == nil {
		h.state = make(map[string]string)
	}
	h.state[key] = truncate(value, 2000)
}

// State returns a snapshot of the latest runtime state map.
func (h *Hub) State() map[string]string {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return cloneMeta(h.state)
}

// Stats returns aggregate counters.
func (h *Hub) Stats() Stats {
	if h == nil {
		return Stats{ByKind: map[Kind]int{}}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	byKind := make(map[Kind]int, len(h.byKind))
	for k, v := range h.byKind {
		byKind[k] = v
	}
	return Stats{
		TotalEntries:   len(h.entries),
		ByKind:         byKind,
		ErrorCount:     h.errors,
		LastActivityAt: h.lastAt,
		StateKeys:      len(h.state),
	}
}

// TailQuery filters stored entries newest-first up to limit.
type TailQuery struct {
	Kind   Kind
	Level  Level
	Since  time.Time
	Limit  int
	Errors bool
}

// Tail returns entries matching q, newest first.
func (h *Hub) Tail(q TailQuery) []Entry {
	if h == nil {
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
		if q.Errors && e.Level != LevelError && e.Level != LevelWarn {
			continue
		}
		if !q.Since.IsZero() && e.At.Before(q.Since) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func (h *Hub) mergeStateLocked(meta map[string]string) {
	if len(meta) == 0 {
		return
	}
	if h.state == nil {
		h.state = make(map[string]string)
	}
	for k, v := range meta {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		h.state[k] = truncate(v, 2000)
	}
}

func (h *Hub) trimLocked() {
	max := h.limits.MaxEntries
	if max <= 0 || len(h.entries) <= max {
		return
	}
	drop := len(h.entries) - max
	h.entries = append([]Entry(nil), h.entries[drop:]...)
	// Recompute kind/error counts after trim — rare path.
	h.byKind = make(map[Kind]int)
	h.errors = 0
	for _, e := range h.entries {
		h.byKind[e.Kind]++
		if e.Level == LevelError {
			h.errors++
		}
	}
}

func cloneMeta(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}

// FormatStateLines renders state keys in stable order.
func FormatStateLines(state map[string]string) []string {
	if len(state) == 0 {
		return nil
	}
	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+state[k])
	}
	return lines
}
