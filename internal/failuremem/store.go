package failuremem

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"arcdesk/internal/repomap"
)

const fileName = "failure-memory.jsonl"

// Entry is one recorded failure and its fix.
type Entry struct {
	TS         time.Time `json:"ts"`
	Signature  string    `json:"signature"`
	Error      string    `json:"error"`
	Fix        string    `json:"fix"`
	Paths      []string  `json:"paths,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	Kind       string    `json:"kind,omitempty"`
	Confidence string    `json:"confidence,omitempty"`
	Hits       int       `json:"hits,omitempty"`
	LastUsedAt           time.Time `json:"last_used_at,omitempty"`
	ID                   string    `json:"id,omitempty"`
	RepoHead             string    `json:"repo_head,omitempty"`
	WorkspaceFingerprint string    `json:"workspace_fingerprint,omitempty"`
}

// Store appends and searches failure memory for a workspace.
type Store struct {
	root       string
	maxEntries int
	mu         sync.Mutex
	entries    []Entry
	loaded     bool
}

// Open returns a store bound to workspace root.
func Open(root string, maxEntries int) (*Store, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("empty workspace")
	}
	if maxEntries <= 0 {
		maxEntries = 500
	}
	if _, err := repomap.ProjectDir(root); err != nil {
		return nil, err
	}
	s := &Store{root: root, maxEntries: maxEntries}
	if err := s.CompactDuplicates(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) path() (string, error) {
	dir, err := repomap.ProjectDir(s.root)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// WorkspaceRoot returns the bound workspace directory.
func (s *Store) WorkspaceRoot() string {
	if s == nil {
		return ""
	}
	return s.root
}

// Record appends or merges an entry (truncating old rows when over maxEntries).
func (s *Store) Record(e Entry) error {
	if s == nil {
		return fmt.Errorf("failure memory not configured")
	}
	NormalizeEntry(&e)
	if e.Signature == "" || e.Fix == "" {
		return fmt.Errorf("signature and fix are required")
	}
	s.StampProvenance(&e)
	if len(e.Error) > 2000 {
		e.Error = e.Error[:1997] + "..."
	}
	if len(e.Fix) > 2000 {
		e.Fix = e.Fix[:1997] + "..."
	}
	fp := Fingerprint(e)
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.loadLocked()
	if err != nil {
		return err
	}
	for i := range entries {
		if Fingerprint(entries[i]) != fp {
			continue
		}
		MergeInto(&entries[i], e)
		return s.saveLocked(entries)
	}
	if e.Hits <= 0 {
		e.Hits = 1
	}
	e.LastUsedAt = e.TS
	entries = append(entries, e)
	if len(entries) > s.maxEntries {
		entries = entries[len(entries)-s.maxEntries:]
	}
	return s.saveLocked(entries)
}

// MarkStale marks an entry non-injectable by id or signature prefix.
func (s *Store) MarkStale(idOrSig string) error {
	if s == nil {
		return fmt.Errorf("failure memory not configured")
	}
	idOrSig = strings.TrimSpace(strings.ToLower(idOrSig))
	if idOrSig == "" {
		return fmt.Errorf("id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.loadLocked()
	if err != nil {
		return err
	}
	changed := false
	for i := range entries {
		id := strings.ToLower(strings.TrimSpace(entries[i].ID))
		if id == idOrSig {
			entries[i].Confidence = ConfidenceStale
			changed = true
			break
		}
	}
	if !changed {
		for i := range entries {
			sig := strings.ToLower(strings.TrimSpace(entries[i].Signature))
			if sig == idOrSig || strings.HasPrefix(sig, idOrSig) {
				entries[i].Confidence = ConfidenceStale
				changed = true
				break
			}
		}
	}
	if !changed {
		return fmt.Errorf("entry not found")
	}
	return s.saveLocked(entries)
}

// Touch records a successful retrieval for scoring.
func (s *Store) Touch(fp string) error {
	if s == nil || strings.TrimSpace(fp) == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.loadLocked()
	if err != nil {
		return err
	}
	for i := range entries {
		if Fingerprint(entries[i]) != fp {
			continue
		}
		entries[i].Hits++
		entries[i].LastUsedAt = time.Now().UTC()
		return s.saveLocked(entries)
	}
	return nil
}

// CompactDuplicates merges rows that share a fingerprint (newest wins body).
func (s *Store) CompactDuplicates() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.loadLocked()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	byFP := map[string]Entry{}
	order := []string{}
	for _, e := range entries {
		NormalizeEntry(&e)
		fp := Fingerprint(e)
		if prev, ok := byFP[fp]; ok {
			MergeInto(&prev, e)
			prev.Hits += e.Hits
			if prev.Hits <= 0 {
				prev.Hits = 2
			}
			byFP[fp] = prev
			continue
		}
		if e.Hits <= 0 {
			e.Hits = 1
		}
		byFP[fp] = e
		order = append(order, fp)
	}
	if len(order) == len(entries) {
		return nil
	}
	out := make([]Entry, 0, len(byFP))
	seen := map[string]bool{}
	for _, fp := range order {
		if seen[fp] {
			continue
		}
		seen[fp] = true
		out = append(out, byFP[fp])
	}
	return s.saveLocked(out)
}

type scoredEntry struct {
	e         Entry
	score     float64
	textScore float64
}

// RankedSearch scores entries for knowledge injection (legacy callers; no provenance filter).
func (s *Store) RankedSearch(query string, paths []string, limit int) ([]Entry, error) {
	if s == nil {
		return nil, fmt.Errorf("failure memory not configured")
	}
	if limit <= 0 {
		limit = 3
	}
	entries, err := s.List(s.maxEntries)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var ranked []scoredEntry
	for _, e := range entries {
		NormalizeEntry(&e)
		if !e.IsInjectable() {
			continue
		}
		sc := scoreEntry(e, q, paths)
		if sc <= 0 {
			continue
		}
		ranked = append(ranked, scoredEntry{e: e, score: sc})
	}
	sortEntriesByScore(ranked)
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]Entry, len(ranked))
	for i, r := range ranked {
		out[i] = r.e
	}
	return out, nil
}

// RankedSearchWithContext scores entries and skips provenance-stale rows.
func (s *Store) RankedSearchWithContext(ctx SearchContext, query string, paths []string, limit int) ([]Entry, error) {
	if s == nil {
		return nil, fmt.Errorf("failure memory not configured")
	}
	if limit <= 0 {
		limit = 3
	}
	ctx = ctx.withDefaults()
	entries, err := s.List(s.maxEntries)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var ranked []scoredEntry
	for _, e := range entries {
		NormalizeEntry(&e)
		if !e.IsInjectable() {
			continue
		}
		if st := e.ProvenanceStatus(ctx); !st.AutoInjectable {
			continue
		}
		sc := scoreEntry(e, q, paths)
		if sc <= 0 {
			continue
		}
		ranked = append(ranked, scoredEntry{e: e, score: sc})
	}
	sortEntriesByScore(ranked)
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]Entry, len(ranked))
	for i, r := range ranked {
		out[i] = r.e
	}
	return out, nil
}

func scoreEntry(e Entry, query string, paths []string) float64 {
	if !e.IsInjectable() {
		return 0
	}
	score := scoreEntryText(e, query)
	for _, p := range paths {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		for _, ep := range e.Paths {
			if strings.Contains(strings.ToLower(ep), p) || strings.Contains(p, strings.ToLower(ep)) {
				score += 0.35
				break
			}
		}
	}
	if e.Hits > 0 {
		score += float64(e.Hits) * 0.05
	}
	if e.Confidence == ConfidenceUserConfirmed {
		score += 0.2
	} else if e.Confidence == ConfidenceVerified {
		score += 0.1
	} else if e.Confidence == ConfidenceDraft {
		score *= 0.5
	}
	return score
}

func scoreEntryText(e Entry, query string) float64 {
	if !e.IsInjectable() {
		return 0
	}
	q := strings.ToLower(strings.TrimSpace(query))
	score := 0.0
	if q != "" {
		low := strings.ToLower(e.Signature + " " + e.Error + " " + e.Fix)
		if strings.Contains(low, q) {
			score += 0.5
		}
		for _, tok := range strings.Fields(query) {
			if len(tok) < 2 {
				continue
			}
			if strings.Contains(low, strings.ToLower(tok)) {
				score += 0.25
			}
		}
	}
	return score
}

func sortEntriesByScore(ranked []scoredEntry) {
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})
}

// List returns the most recent entries (newest last in slice).
func (s *Store) List(limit int) ([]Entry, error) {
	if s == nil {
		return nil, fmt.Errorf("failure memory not configured")
	}
	if limit <= 0 {
		limit = 20
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	out := append([]Entry(nil), entries...)
	return out, nil
}

// Search finds entries whose signature, error, fix, paths, or tags match query.
func (s *Store) Search(query string, limit int) ([]Entry, error) {
	if s == nil {
		return nil, fmt.Errorf("failure memory not configured")
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return s.List(limit)
	}
	entries, err := s.List(s.maxEntries)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	var out []Entry
	for i := len(entries) - 1; i >= 0 && len(out) < limit; i-- {
		if entryMatches(entries[i], query) {
			out = append(out, entries[i])
		}
	}
	return out, nil
}

func entryMatches(e Entry, q string) bool {
	if strings.Contains(strings.ToLower(e.Signature), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Error), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Fix), q) {
		return true
	}
	for _, p := range e.Paths {
		if strings.Contains(strings.ToLower(p), q) {
			return true
		}
	}
	for _, tag := range e.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}
	return false
}

func (s *Store) ensureLoadedLocked() error {
	if s.loaded {
		return nil
	}
	entries, err := s.readFromDisk()
	if err != nil {
		return err
	}
	s.entries = entries
	s.loaded = true
	return nil
}

func (s *Store) loadLocked() ([]Entry, error) {
	if err := s.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	return append([]Entry(nil), s.entries...), nil
}

func (s *Store) readFromDisk() ([]Entry, error) {
	p, err := s.path()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("failure memory path is a directory")
	}
	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, sc.Err()
}

func (s *Store) saveLocked(entries []Entry) error {
	s.entries = append([]Entry(nil), entries...)
	s.loaded = true
	p, err := s.path()
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
