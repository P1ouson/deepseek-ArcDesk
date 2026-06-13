package archrag

import (
	"os"
	"path/filepath"
	"strings"
)

var defaultDocNames = []string{
	"docs/SPEC.md",
	"SPEC.md",
	"SECURITY.md",
	"README.md",
	"CONTRIBUTING.md",
	"docs/MIGRATING.md",
	"docs/WAILS_CALLGRAPH.md",
}

const maxSectionBytes = 32 * 1024

// Section is a markdown heading block within a document.
type Section struct {
	Doc     string `json:"doc"`
	Level   int    `json:"level"`
	Heading string `json:"heading"`
	Line    int    `json:"line"`
	Body    string `json:"body"`
}

// DocSummary lists a document and its section headings.
type DocSummary struct {
	Path     string   `json:"path"`
	Title    string   `json:"title"`
	Sections []string `json:"sections"`
	Bytes    int      `json:"bytes"`
}

// Index holds parsed architecture documents for a workspace.
type Index struct {
	root     string
	sections []Section
	summary  []DocSummary
}

// NewIndex loads default and extra architecture documents from workspace root.
func NewIndex(root string, extraPaths []string) *Index {
	root = strings.TrimSpace(root)
	paths := append([]string{}, defaultDocNames...)
	for _, p := range extraPaths {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	seen := map[string]bool{}
	idx := &Index{root: root}
	for _, rel := range paths {
		rel = filepath.ToSlash(rel)
		if seen[rel] {
			continue
		}
		seen[rel] = true
		abs := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		text := string(data)
		parsed := parseSections(rel, text)
		if len(parsed) == 0 {
			parsed = []Section{{Doc: rel, Level: 1, Heading: filepath.Base(rel), Line: 1, Body: truncate(text, maxSectionBytes)}}
		}
		idx.sections = append(idx.sections, parsed...)
		var headings []string
		for _, s := range parsed {
			headings = append(headings, s.Heading)
		}
		title := parsed[0].Heading
		idx.summary = append(idx.summary, DocSummary{
			Path: rel, Title: title, Sections: headings, Bytes: len(data),
		})
	}
	return idx
}

func parseSections(doc, text string) []Section {
	lines := strings.Split(text, "\n")
	var res []Section
	var cur *Section
	curLine := 0
	for i, line := range lines {
		curLine = i + 1
		level := headingLevel(line)
		if level == 0 {
			if cur != nil {
				cur.Body += line + "\n"
			}
			continue
		}
		if cur != nil {
			cur.Body = strings.TrimSpace(truncate(cur.Body, maxSectionBytes))
			res = append(res, *cur)
		}
		cur = &Section{
			Doc:     doc,
			Level:   level,
			Heading: strings.TrimSpace(strings.TrimLeft(line, "#")),
			Line:    curLine,
		}
	}
	if cur != nil {
		cur.Body = strings.TrimSpace(truncate(cur.Body, maxSectionBytes))
		res = append(res, *cur)
	}
	return res
}

func headingLevel(line string) int {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "#") {
		return 0
	}
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n >= len(line) || line[n] != ' ' {
		return 0
	}
	return n
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n… [truncated]"
}

// List returns document summaries.
func (i *Index) List() []DocSummary {
	if i == nil {
		return nil
	}
	return append([]DocSummary(nil), i.summary...)
}

// Sections returns all parsed architecture sections.
func (i *Index) Sections() []Section {
	if i == nil {
		return nil
	}
	return append([]Section(nil), i.sections...)
}

// FindSections returns sections whose heading or body matches query (case-insensitive).
func (i *Index) FindSections(doc, query string, limit int) []Section {
	if i == nil {
		return nil
	}
	if limit <= 0 {
		limit = 5
	}
	doc = strings.TrimSpace(filepath.ToSlash(doc))
	query = strings.ToLower(strings.TrimSpace(query))
	var out []Section
	for _, s := range i.sections {
		if doc != "" && s.Doc != doc {
			continue
		}
		if query != "" {
			hay := strings.ToLower(s.Heading + "\n" + s.Body)
			if !strings.Contains(hay, query) {
				continue
			}
		}
		out = append(out, s)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// ReadDoc returns the full text of an indexed document if present.
func (i *Index) ReadDoc(doc string) (string, bool) {
	if i == nil {
		return "", false
	}
	doc = strings.TrimSpace(filepath.ToSlash(doc))
	abs := filepath.Join(i.root, filepath.FromSlash(doc))
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", false
	}
	for _, s := range i.summary {
		if s.Path == doc {
			return truncate(string(data), maxSectionBytes*2), true
		}
	}
	return "", false
}
