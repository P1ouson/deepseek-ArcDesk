// Package toolcache provides a session-scoped result cache for read-only tools.
// Phase 1 skips re-execution when the same (tool, canonical args) was already
// run successfully in this session; Phase 2 normalizes equivalent args (path
// aliases, default fields) before lookup. Writer success clears the cache.
package toolcache

import (
	"sync"

	"arcdesk/internal/toolstats"
)

// Entry is a cached tool outcome fed back to the model.
type Entry struct {
	Output    string
	ErrMsg    string
	Truncated bool
	TruncMsg  string
}

// Stats counts cache lookups this session and current user turn.
type Stats struct {
	SessionHits   int
	SessionMisses int
	TurnHits      int
	TurnMisses    int
}

// Cache stores successful read-only tool results for the agent session.
type Cache struct {
	mu sync.Mutex

	entries map[string]cachedEntry
	stats   Stats
	keyCtx  toolstats.KeyContext
}

type cachedEntry struct {
	out   Entry
	paths []string
}

// New returns an empty session cache.
func New() *Cache {
	return &Cache{entries: make(map[string]cachedEntry)}
}

// SetKeyContext configures Phase-2 workspace-aware arg normalization for keys.
func (c *Cache) SetKeyContext(ctx toolstats.KeyContext) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keyCtx = ctx
}

func (c *Cache) cacheKey(name, argsJSON string) string {
	return toolstats.IntentKey(name, argsJSON, c.keyCtx)
}

// ResetTurn clears per-user-turn hit/miss counters. Cached entries persist.
func (c *Cache) ResetTurn() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.TurnHits = 0
	c.stats.TurnMisses = 0
}

// Cacheable reports whether a tool's successful result may be cached.
func Cacheable(name string, readOnly bool) bool {
	if !readOnly {
		return false
	}
	switch name {
	case "complete_step", "todo_write", "ask", "task":
		return false
	}
	return true
}

// Get returns a cached entry and records a hit. ok is false on miss.
func (c *Cache) Get(name, argsJSON string) (Entry, bool) {
	if c == nil {
		return Entry{}, false
	}
	key := c.cacheKey(name, argsJSON)
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if ok {
		c.stats.SessionHits++
		c.stats.TurnHits++
	}
	return e.out, ok
}

// RecordMiss counts a cacheable lookup that did not hit.
func (c *Cache) RecordMiss() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.SessionMisses++
	c.stats.TurnMisses++
}

// Put stores a successful tool outcome. Errors and blocked calls must not be cached.
func (c *Cache) Put(name, argsJSON string, e Entry) {
	if c == nil {
		return
	}
	key := c.cacheKey(name, argsJSON)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cachedEntry{
		out:   e,
		paths: CachePaths(name, argsJSON, c.keyCtx),
	}
}

// InvalidateAll drops every entry — fallback when write paths are unknown.
func (c *Cache) InvalidateAll() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]cachedEntry)
}

// InvalidatePaths drops entries whose cached paths overlap written paths.
// Returns the number of entries removed; falls back to InvalidateAll when
// writtenPaths is empty.
func (c *Cache) InvalidatePaths(writtenPaths []string) int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(writtenPaths) == 0 {
		n := len(c.entries)
		c.entries = make(map[string]cachedEntry)
		return n
	}
	normalized := make([]string, 0, len(writtenPaths))
	for _, p := range writtenPaths {
		if p = normalizeSlash(p); p != "" {
			normalized = append(normalized, p)
		}
	}
	if len(normalized) == 0 {
		n := len(c.entries)
		c.entries = make(map[string]cachedEntry)
		return n
	}
	removed := 0
	for key, rec := range c.entries {
		if entryAffectedByWrites(rec.paths, normalized) {
			delete(c.entries, key)
			removed++
		}
	}
	return removed
}

// Snapshot returns a copy of hit/miss counters.
func (c *Cache) Snapshot() Stats {
	if c == nil {
		return Stats{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}
