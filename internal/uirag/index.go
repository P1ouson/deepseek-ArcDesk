package uirag

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	exportFuncRe  = regexp.MustCompile(`export\s+(?:default\s+)?function\s+([A-Z][A-Za-z0-9_]*)`)
	exportConstRe = regexp.MustCompile(`export\s+const\s+([A-Z][A-Za-z0-9_]*)\s*[=:]`)
)

// Component is one discovered UI module export.
type Component struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Rel  string `json:"rel"`
}

// Index holds scanned frontend components under a project root.
type Index struct {
	Root       string      `json:"root"`
	ScanRoots  []string    `json:"scan_roots"`
	Components []Component `json:"components"`
}

// Discoverable reports whether the project has a scannable frontend tree.
func Discoverable(root string) bool {
	for _, p := range scanRoots(root) {
		if dirHasUIFiles(p) {
			return true
		}
	}
	return false
}

func dirHasUIFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".tsx" || ext == ".jsx" || ext == ".vue" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func scanRoots(root string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	candidates := []string{
		filepath.Join(root, "desktop", "frontend", "src"),
		filepath.Join(root, "frontend", "src"),
		filepath.Join(root, "ui", "src"),
	}
	var out []string
	seen := map[string]bool{}
	for _, p := range candidates {
		p = filepath.Clean(p)
		if seen[p] {
			continue
		}
		seen[p] = true
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// BuildIndex walks frontend source trees and extracts exported component names.
func BuildIndex(root string) *Index {
	roots := scanRoots(root)
	idx := &Index{Root: root, ScanRoots: roots}
	seen := map[string]bool{}
	for _, scanRoot := range roots {
		_ = filepath.WalkDir(scanRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".tsx" && ext != ".jsx" {
				return nil
			}
			if strings.Contains(path, "node_modules") || strings.Contains(path, ".test.") || strings.Contains(path, "__tests__") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			names := extractNames(string(b))
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			for _, name := range names {
				key := rel + ":" + name
				if seen[key] {
					continue
				}
				seen[key] = true
				idx.Components = append(idx.Components, Component{Name: name, Path: path, Rel: rel})
			}
			return nil
		})
	}
	sort.Slice(idx.Components, func(i, j int) bool {
		if idx.Components[i].Rel == idx.Components[j].Rel {
			return idx.Components[i].Name < idx.Components[j].Name
		}
		return idx.Components[i].Rel < idx.Components[j].Rel
	})
	return idx
}

func extractNames(src string) []string {
	seen := map[string]bool{}
	var out []string
	for _, re := range []*regexp.Regexp{exportFuncRe, exportConstRe} {
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			if len(m) < 2 {
				continue
			}
			name := strings.TrimSpace(m[1])
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// Find returns components whose name or path contains query (case-insensitive).
func (idx *Index) Find(query string, limit int) []Component {
	if idx == nil {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if limit <= 0 {
		limit = 50
	}
	var out []Component
	for _, c := range idx.Components {
		if q != "" &&
			!strings.Contains(strings.ToLower(c.Name), q) &&
			!strings.Contains(strings.ToLower(c.Rel), q) {
			continue
		}
		out = append(out, c)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// Lookup returns the first component matching name or relative path.
func (idx *Index) Lookup(nameOrPath string) (Component, bool) {
	if idx == nil {
		return Component{}, false
	}
	key := strings.ToLower(strings.TrimSpace(nameOrPath))
	for _, c := range idx.Components {
		if strings.EqualFold(c.Name, key) ||
			strings.EqualFold(c.Rel, key) ||
			strings.EqualFold(filepath.ToSlash(c.Path), key) {
			return c, true
		}
	}
	return Component{}, false
}
