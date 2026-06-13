package failuremem

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"arcdesk/internal/repomap"
)

const fileName = "failure-memory.jsonl"

// Entry is one recorded failure and its fix.
type Entry struct {
	TS        time.Time `json:"ts"`
	Signature string    `json:"signature"`
	Error     string    `json:"error"`
	Fix       string    `json:"fix"`
	Paths     []string  `json:"paths,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
}

// Store appends and searches failure memory for a workspace.
type Store struct {
	root       string
	maxEntries int
	mu         sync.Mutex
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
	return &Store{root: root, maxEntries: maxEntries}, nil
}

func (s *Store) path() (string, error) {
	dir, err := repomap.ProjectDir(s.root)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// Record appends an entry (truncating old rows when over maxEntries).
func (s *Store) Record(e Entry) error {
	if s == nil {
		return fmt.Errorf("failure memory not configured")
	}
	e.Signature = strings.TrimSpace(e.Signature)
	e.Error = strings.TrimSpace(e.Error)
	e.Fix = strings.TrimSpace(e.Fix)
	if e.Signature == "" || e.Fix == "" {
		return fmt.Errorf("signature and fix are required")
	}
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	}
	if len(e.Error) > 2000 {
		e.Error = e.Error[:1997] + "..."
	}
	if len(e.Fix) > 2000 {
		e.Fix = e.Fix[:1997] + "..."
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.loadLocked()
	if err != nil {
		return err
	}
	entries = append(entries, e)
	if len(entries) > s.maxEntries {
		entries = entries[len(entries)-s.maxEntries:]
	}
	return s.saveLocked(entries)
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

func (s *Store) loadLocked() ([]Entry, error) {
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
