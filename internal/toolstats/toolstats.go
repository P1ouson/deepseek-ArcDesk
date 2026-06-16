// Package toolstats records duplicate tool-call patterns for Phase-0 reuse
// measurement. It does not cache or skip execution — only counts (tool, args)
// repeats so later phases can attach a real tool-result cache.
package toolstats

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

// Stats is a snapshot of tool reuse counters.
type Stats struct {
	Calls           int // total tool dispatches counted
	Duplicates      int // repeat (tool+args) calls beyond the first
	CacheableCalls  int // read-only tool dispatches
	CacheableDupes  int // repeat read-only dispatches beyond the first
	NormalizedDupes int // repeats visible only after Phase-2 arg normalization
}

// DuplicateRate returns the fraction of calls that were repeats (0 when Calls==0).
func (s Stats) DuplicateRate() float64 {
	if s.Calls <= 0 {
		return 0
	}
	return float64(s.Duplicates) / float64(s.Calls)
}

// CacheableDuplicateRate returns repeat rate among cacheable calls only.
func (s Stats) CacheableDuplicateRate() float64 {
	if s.CacheableCalls <= 0 {
		return 0
	}
	return float64(s.CacheableDupes) / float64(s.CacheableCalls)
}

// Tracker accumulates per-user-turn and session-wide tool dispatch stats.
type Tracker struct {
	mu sync.Mutex

	turnKeys map[string]int
	sessKeys map[string]int

	turn Stats
	sess Stats

	keyCtx KeyContext
}

// NewTracker returns a ready tracker.
func NewTracker() *Tracker {
	return &Tracker{
		turnKeys: make(map[string]int),
		sessKeys: make(map[string]int),
	}
}

// ResetTurn clears per-user-turn counters. Session totals are preserved.
func (t *Tracker) ResetTurn() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.turnKeys = make(map[string]int)
	t.turn = Stats{}
}

// SetKeyContext configures Phase-2 workspace-aware normalization for reuse keys.
func (t *Tracker) SetKeyContext(ctx KeyContext) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.keyCtx = ctx
}

// Record counts one tool dispatch. cacheable should be true for read-only tools
// that are candidates for a future tool-result cache.
func (t *Tracker) Record(name, argsJSON string, cacheable bool) {
	if t == nil || strings.TrimSpace(name) == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	ctx := t.keyCtx
	key := IntentKey(name, argsJSON, ctx)
	rawKey := Key(name, argsJSON)
	normalizedAlias := ctx.Normalize && rawKey != key
	t.recordLocked(&t.turn, t.turnKeys, key, cacheable, normalizedAlias)
	t.recordLocked(&t.sess, t.sessKeys, key, cacheable, normalizedAlias)
}

func (t *Tracker) recordLocked(s *Stats, keys map[string]int, key string, cacheable, normalizedAlias bool) {
	s.Calls++
	if cacheable {
		s.CacheableCalls++
	}
	n := keys[key]
	keys[key] = n + 1
	if n > 0 {
		s.Duplicates++
		if cacheable {
			s.CacheableDupes++
		}
		if normalizedAlias {
			s.NormalizedDupes++
		}
	}
}

// Turn returns a copy of the current user-turn stats.
func (t *Tracker) Turn() Stats {
	if t == nil {
		return Stats{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.turn
}

// Session returns a copy of session-cumulative stats.
func (t *Tracker) Session() Stats {
	if t == nil {
		return Stats{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sess
}

// Key builds a stable (tool, canonical-args) identity for duplicate detection.
func Key(name, argsJSON string) string {
	return name + "\x00" + CanonicalArgs(argsJSON)
}

// CanonicalArgs normalizes JSON arguments so key order and whitespace do not
// split duplicates. Non-JSON args fall back to trimmed raw text.
func CanonicalArgs(argsJSON string) string {
	raw := strings.TrimSpace(argsJSON)
	if raw == "" {
		return ""
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	return string(canonicalJSON(v))
}

func canonicalJSON(v any) []byte {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b bytes.Buffer
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			b.Write(kb)
			b.WriteByte(':')
			b.Write(canonicalJSON(x[k]))
		}
		b.WriteByte('}')
		return b.Bytes()
	case []any:
		var b bytes.Buffer
		b.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			b.Write(canonicalJSON(item))
		}
		b.WriteByte(']')
		return b.Bytes()
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return []byte("null")
		}
		return b
	}
}
