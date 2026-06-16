// Package plancache stores reusable task-decomposition skeletons keyed by
// intent + repo HEAD (Phase 5). It caches subtask structure, not final patches.
package plancache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"arcdesk/internal/intent"
	"arcdesk/internal/planner"
	"arcdesk/internal/repomap"
)

const cacheFileName = "plan-cache.json"

// Phase is a cached plan stage (title + optional sub-steps).
type Phase struct {
	Title string   `json:"title"`
	Steps []string `json:"steps,omitempty"`
}

// Entry is one cached decomposition skeleton.
type Entry struct {
	Intent     string    `json:"intent"`
	RepoHead   string    `json:"repoHead"`
	Phases     []Phase   `json:"phases"`
	ToolHints  []string  `json:"toolHints,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
	Hits       int       `json:"hits"`
}

// Settings controls lookup/store policy.
type Settings struct {
	MinConfidence float64
	TTLDays       int
	MinPhases     int
}

func (s Settings) withDefaults() Settings {
	if s.MinConfidence <= 0 {
		s.MinConfidence = 0.8
	}
	if s.TTLDays <= 0 {
		s.TTLDays = 30
	}
	if s.MinPhases <= 0 {
		s.MinPhases = 2
	}
	return s
}

// Store persists plan skeletons under <workspace>/.arcdesk/plan-cache.json.
type Store struct {
	mu           sync.Mutex
	root         string
	cfg          Settings
	entries      map[string]Entry
	dirty        bool
	lookupHits   int
	lookupMisses int
	flushTimer   *time.Timer
}

type filePayload struct {
	Version int              `json:"version"`
	Entries map[string]Entry `json:"entries"`
}

// Open loads or creates the workspace plan cache.
func Open(workspaceRoot string, cfg Settings) (*Store, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return nil, fmt.Errorf("workspace root required")
	}
	s := &Store{
		root:    root,
		cfg:     cfg.withDefaults(),
		entries: make(map[string]Entry),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func cacheKey(intentClass, repoHead string) string {
	return strings.ToLower(strings.TrimSpace(intentClass)) + "\x00" + strings.TrimSpace(repoHead)
}

// Lookup returns a planner hint block when a fresh skeleton exists.
func (s *Store) Lookup(in intent.Result, workspaceRoot string) (hint string, hit bool) {
	if s == nil || !cacheableIntent(in) {
		return "", false
	}
	if in.Confidence < s.cfg.MinConfidence {
		return "", false
	}
	head, _ := repomap.WorkspaceRevision(workspaceRoot)
	if head == "" {
		return "", false
	}
	key := cacheKey(in.Canonical, head)
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[key]
	if !ok {
		s.lookupMisses++
		return "", false
	}
	if s.cfg.TTLDays > 0 && !e.CreatedAt.IsZero() {
		if time.Since(e.CreatedAt.UTC()) > time.Duration(s.cfg.TTLDays)*24*time.Hour {
			s.lookupMisses++
			return "", false
		}
	}
	if len(e.Phases) < s.cfg.MinPhases {
		s.lookupMisses++
		return "", false
	}
	e.Hits++
	e.LastUsedAt = time.Now().UTC()
	s.entries[key] = e
	s.dirty = true
	s.lookupHits++
	s.scheduleFlushLocked()
	return formatHint(in.Canonical, head, e), true
}

// Record extracts phases from a planner output and stores when eligible.
func (s *Store) Record(in intent.Result, workspaceRoot, planText string) {
	if s == nil || !cacheableIntent(in) {
		return
	}
	if in.Confidence < s.cfg.MinConfidence {
		return
	}
	phases := extractPhases(planText)
	if len(phases) < s.cfg.MinPhases {
		return
	}
	head, _ := repomap.WorkspaceRevision(workspaceRoot)
	if head == "" {
		return
	}
	key := cacheKey(in.Canonical, head)
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.entries[key]
	e := Entry{
		Intent:     in.Canonical,
		RepoHead:   head,
		Phases:     phases,
		ToolHints:  extractToolHints(planText),
		CreatedAt:  now,
		LastUsedAt: now,
	}
	if !prev.CreatedAt.IsZero() {
		e.CreatedAt = prev.CreatedAt
	}
	s.entries[key] = e
	s.dirty = true
	s.scheduleFlushLocked()
}

// Flush persists pending changes immediately.
func (s *Store) Flush() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) scheduleFlushLocked() {
	if s.flushTimer != nil {
		s.flushTimer.Stop()
	}
	s.flushTimer = time.AfterFunc(5*time.Second, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		_ = s.saveLocked()
	})
}

// Stats returns session-visible hit/miss counters (in-memory for tests).
func (s *Store) Stats() (hits, entries int) {
	if s == nil {
		return 0, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.entries {
		hits += e.Hits
	}
	return hits, len(s.entries)
}

func cacheableIntent(in intent.Result) bool {
	switch in.Canonical {
	case intent.ClassWrite, intent.ClassQA, intent.ClassGeneral:
		return false
	default:
		return in.Canonical != ""
	}
}

func extractPhases(plan string) []Phase {
	raw := planner.ParsePhases(plan)
	out := make([]Phase, 0, len(raw))
	for _, p := range raw {
		title := strings.TrimSpace(p.Title)
		if title == "" {
			continue
		}
		cp := Phase{Title: title}
		for _, step := range p.Steps {
			if step = strings.TrimSpace(step); step != "" {
				cp.Steps = append(cp.Steps, step)
			}
		}
		out = append(out, cp)
	}
	return out
}

func extractToolHints(plan string) []string {
	lower := strings.ToLower(plan)
	var hints []string
	candidates := []string{
		"read_file", "grep", "glob", "go test", "dependency_", "callgraph_",
		"codegraph_", "run_skill", "bash",
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		if strings.Contains(lower, c) && !seen[c] {
			seen[c] = true
			hints = append(hints, c)
		}
	}
	return hints
}

func formatHint(intentClass, head string, e Entry) string {
	var b strings.Builder
	b.WriteString("[plan-cache hint intent=")
	b.WriteString(intentClass)
	b.WriteString(" head=")
	b.WriteString(shortHead(head))
	b.WriteString("]\n")
	b.WriteString("Previous similar task on this repo used this subtask skeleton (adapt as needed):\n")
	for i, p := range e.Phases {
		fmt.Fprintf(&b, "%d. %s\n", i+1, p.Title)
		for _, step := range p.Steps {
			b.WriteString("   - ")
			b.WriteString(step)
			b.WriteByte('\n')
		}
	}
	if len(e.ToolHints) > 0 {
		b.WriteString("Tool hints: ")
		b.WriteString(strings.Join(e.ToolHints, ", "))
		b.WriteByte('\n')
	}
	b.WriteString("Do not treat as gospel — verify against the current task and codebase.")
	return b.String()
}

func shortHead(h string) string {
	h = strings.TrimSpace(h)
	if len(h) <= 7 {
		return h
	}
	return h[:7]
}

func (s *Store) load() error {
	dir, err := repomap.ProjectDir(s.root)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, cacheFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var payload filePayload
	if err := json.Unmarshal(b, &payload); err != nil {
		return err
	}
	if payload.Entries != nil {
		s.entries = payload.Entries
	}
	return nil
}

func (s *Store) saveLocked() error {
	if !s.dirty {
		return nil
	}
	dir, err := repomap.ProjectDir(s.root)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, cacheFileName)
	payload := filePayload{Version: 1, Entries: s.entries}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	s.dirty = false
	return nil
}
