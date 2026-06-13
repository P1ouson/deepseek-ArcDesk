// Package repomap maintains a project-level repository map and shared read-index
// under <workspace>/.arcdesk/. The map is folded into the cache-stable system
// prefix at boot so new sessions reuse the same bytes across topics.
package repomap

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"arcdesk/internal/proc"
)

const (
	subdir           = ".arcdesk"
	mapFileName         = "repo-map.md"
	exploreSummariesName = "explore-summaries.md"
	metaFileName        = "repo-map.meta.json"
	readIndexName    = "read-index.json"
	maxReadIndexRows = 40
	mapHeader        = "# Project repository map (shared)\n\n"
)

// Meta records when the repo map was generated and what revision it reflects.
type Meta struct {
	GeneratedAt string `json:"generatedAt"`
	GitHead     string `json:"gitHead,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// ReadEntry is one shared cross-session read_file receipt.
type ReadEntry struct {
	Path      string `json:"path"`
	Summary   string `json:"summary,omitempty"`
	ModTime   int64  `json:"modTime,omitempty"`
	UpdatedAt int64  `json:"updatedAt"`
}

// ProjectDir returns <workspace>/.arcdesk (created on demand).
func ProjectDir(workspace string) (string, error) {
	root := strings.TrimSpace(workspace)
	if root == "" {
		return "", fmt.Errorf("workspace required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(abs, subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func mapPath(workspace string) (string, error) {
	dir, err := ProjectDir(workspace)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, mapFileName), nil
}

func metaPath(workspace string) (string, error) {
	dir, err := ProjectDir(workspace)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, metaFileName), nil
}

func readIndexPath(workspace string) (string, error) {
	dir, err := ProjectDir(workspace)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, readIndexName), nil
}

// Compose folds the shared project map onto the system prompt after memory.
// Returns base unchanged when workspace is empty or no map exists.
func Compose(base, workspace string) string {
	block := LoadBlock(workspace)
	if block == "" {
		return base
	}
	if strings.TrimSpace(base) == "" {
		return block
	}
	return strings.TrimRight(base, "\n") + "\n\n" + block
}

// LoadBlock returns the markdown block for prefix injection, or "" if missing.
func LoadBlock(workspace string) string {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return ""
	}
	p, err := mapPath(workspace)
	if err != nil {
		return ""
	}
	var body string
	if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
		body = strings.TrimSpace(string(b))
	}
	if extra := loadExploreSummariesBody(workspace); extra != "" {
		if body != "" {
			body += "\n\n"
		}
		body += extra
	}
	if body == "" {
		return ""
	}
	var out strings.Builder
	out.WriteString(mapHeader)
	out.WriteString("Auto-generated project snapshot shared across sessions in this workspace. ")
	out.WriteString("Use this before glob/grep/ls the whole tree. For symbols/callers prefer codegraph_* when available. ")
	out.WriteString("Verify stale facts with read_file before acting.\n\n")
	if body != "" {
		out.WriteString(body)
		out.WriteString("\n")
	}
	if rows := loadReadIndex(workspace); len(rows) > 0 {
		out.WriteString("\n## Recently read files (shared)\n\n")
		for _, row := range rows {
			if row.Summary != "" {
				out.WriteString(fmt.Sprintf("- `%s` — %s\n", row.Path, row.Summary))
			} else {
				out.WriteString(fmt.Sprintf("- `%s`\n", row.Path))
			}
		}
	}
	out.WriteString("\n## Exploration policy\n\n")
	out.WriteString("- Do NOT re-scan the full repository when this map or AGENTS.md / ARCDESK.md already answers structure questions.\n")
	out.WriteString("- Prefer read_file on specific paths listed here; use grep/glob only for unknowns.\n")
	out.WriteString("- When codegraph MCP tools are connected, use them for definitions/callers before blind grep.\n")
	return out.String()
}

func loadReadIndex(workspace string) []ReadEntry {
	p, err := readIndexPath(workspace)
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil || len(b) == 0 {
		return nil
	}
	var rows []ReadEntry
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil
	}
	if len(rows) > maxReadIndexRows {
		rows = rows[:maxReadIndexRows]
	}
	return rows
}

// RecordRead upserts a shared read receipt for the workspace (cross-tab).
func RecordRead(workspace, path, summary string) error {
	workspace = strings.TrimSpace(workspace)
	path = filepath.ToSlash(strings.TrimSpace(path))
	if workspace == "" || path == "" {
		return nil
	}
	p, err := readIndexPath(workspace)
	if err != nil {
		return err
	}
	rows := loadReadIndex(workspace)
	now := time.Now().UnixMilli()
	summary = strings.TrimSpace(summary)
	if len(summary) > 160 {
		summary = summary[:157] + "..."
	}
	found := false
	for i, row := range rows {
		if row.Path == path {
			rows[i].Summary = summary
			rows[i].UpdatedAt = now
			found = true
			break
		}
	}
	if !found {
		rows = append([]ReadEntry{{Path: path, Summary: summary, UpdatedAt: now}}, rows...)
	}
	if len(rows) > maxReadIndexRows {
		rows = rows[:maxReadIndexRows]
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// RecordExploreSummary appends a distilled explore sub-agent conclusion for reuse.
func RecordExploreSummary(workspace, task, summary string) error {
	task = strings.TrimSpace(task)
	summary = strings.TrimSpace(summary)
	if task == "" || summary == "" {
		return nil
	}
	if len(summary) > 400 {
		summary = summary[:397] + "..."
	}
	if len(task) > 120 {
		task = task[:117] + "..."
	}
	dir, err := ProjectDir(workspace)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, exploreSummariesName)
	line := fmt.Sprintf("- **%s** — %s\n", task, summary)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

func loadExploreSummariesBody(workspace string) string {
	dir, err := ProjectDir(workspace)
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(dir, exploreSummariesName))
	if err != nil || len(b) == 0 {
		return ""
	}
	text := strings.TrimSpace(string(b))
	if text == "" {
		return ""
	}
	return "## Explore conclusions (shared)\n\n" + text
}

var refreshLocks sync.Map

func lockFor(workspace string) *sync.Mutex {
	v, _ := refreshLocks.LoadOrStore(workspace, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// EnsureReady guarantees a repo-map file exists before boot folds the prefix.
// It runs synchronously only when the map is missing; stale maps are left to
// background RefreshIfStale.
func EnsureReady(workspace string) error {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return nil
	}
	mp, err := mapPath(workspace)
	if err != nil {
		return err
	}
	if _, err := os.Stat(mp); err == nil {
		return nil
	}
	mu := lockFor(workspace)
	mu.Lock()
	defer mu.Unlock()
	if _, err := os.Stat(mp); err == nil {
		return nil
	}
	return refresh(workspace)
}

// RefreshIfStale rebuilds the repo map when missing or the repo revision changed.
// Safe to call from background goroutines; concurrent calls for one workspace
// serialize on a per-workspace lock so only one refresh runs at a time.
func RefreshIfStale(workspace string) error {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return nil
	}
	mu := lockFor(workspace)
	mu.Lock()
	defer mu.Unlock()

	stale, err := isStale(workspace)
	if err != nil {
		return err
	}
	if !stale {
		return nil
	}
	return refresh(workspace)
}

func isStale(workspace string) (bool, error) {
	mp, err := mapPath(workspace)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(mp); err != nil {
		return true, nil
	}
	meta, err := loadMeta(workspace)
	if err != nil {
		return true, err
	}
	head, fp := repoRevision(workspace)
	if head != "" {
		return meta.GitHead != head, nil
	}
	if fp == meta.Fingerprint {
		return false, nil
	}
	return true, nil
}

func loadMeta(workspace string) (Meta, error) {
	p, err := metaPath(workspace)
	if err != nil {
		return Meta{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Meta{}, err
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return Meta{}, err
	}
	return m, nil
}

func saveMeta(workspace string, m Meta) error {
	p, err := metaPath(workspace)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func refresh(workspace string) error {
	body, err := generateMap(workspace)
	if err != nil {
		return err
	}
	mp, err := mapPath(workspace)
	if err != nil {
		return err
	}
	tmp := mp + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, mp); err != nil {
		return err
	}
	head, fp := repoRevision(workspace)
	return saveMeta(workspace, Meta{
		GeneratedAt: time.Now().Format(time.RFC3339),
		GitHead:     head,
		Fingerprint: fp,
	})
}

func repoRevision(workspace string) (gitHead, fingerprint string) {
	cmd := exec.Command("git", "-C", workspace, "rev-parse", "HEAD")
	proc.HideWindowDetached(cmd)
	if out, err := cmd.Output(); err == nil {
		gitHead = strings.TrimSpace(string(out))
		if gitHead != "" {
			return gitHead, ""
		}
	}
	// Non-git: fingerprint from top-level mod times.
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return "", ""
	}
	ents, err := os.ReadDir(abs)
	if err != nil {
		return "", ""
	}
	sortEntries(ents)
	var parts []string
	for _, e := range ents {
		name := e.Name()
		if name == subdir {
			continue // our own cache; writing map/meta must not invalidate the fingerprint
		}
		if e.IsDir() && skipDir(name) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%d", e.Name(), info.ModTime().Unix()))
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return "", ""
	}
	return "", strings.Join(parts, "|")
}

func generateMap(workspace string) (string, error) {
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("Workspace: " + abs + "\n\n")

	if readme := readSnippet(filepath.Join(abs, "README.md"), 2500); readme != "" {
		b.WriteString("## README (excerpt)\n\n")
		b.WriteString(readme)
		b.WriteString("\n\n")
	}
	for _, name := range []string{"go.mod", "package.json", "pyproject.toml", "Cargo.toml", "Makefile"} {
		if snippet := readSnippet(filepath.Join(abs, name), 1200); snippet != "" {
			b.WriteString("## " + name + " (excerpt)\n\n")
			b.WriteString(snippet)
			b.WriteString("\n\n")
		}
	}

	b.WriteString("## Top-level layout\n\n")
	lines, err := listTree(abs, 2)
	if err != nil {
		return "", err
	}
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

func readSnippet(path string, max int) string {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return ""
	}
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		s = s[:max] + "\n…"
	}
	return s
}

func listTree(root string, maxDepth int) ([]string, error) {
	var out []string
	var walk func(dir string, depth int, prefix string) error
	walk = func(dir string, depth int, prefix string) error {
		if depth > maxDepth {
			return nil
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		sortEntries(ents)
		for _, e := range ents {
			name := e.Name()
			if skipDir(name) {
				continue
			}
			line := prefix + name
			if e.IsDir() {
				line += "/"
			}
			out = append(out, line)
			if e.IsDir() && depth < maxDepth {
				_ = walk(filepath.Join(dir, name), depth+1, prefix+"  ")
			}
		}
		return nil
	}
	if err := walk(root, 0, ""); err != nil {
		return nil, err
	}
	if len(out) > 120 {
		out = append(out[:120], "… (truncated)")
	}
	return out, nil
}

func skipDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", "node_modules", "vendor", "dist", "build", "out", "target",
		"__pycache__", ".venv", "venv", ".idea", ".vscode", ".cursor":
		return true
	}
	return false
}

func sortEntries(ents []os.DirEntry) {
	sort.Slice(ents, func(i, j int) bool {
		di, dj := ents[i].IsDir(), ents[j].IsDir()
		if di != dj {
			return di
		}
		return strings.ToLower(ents[i].Name()) < strings.ToLower(ents[j].Name())
	})
}
