package constraint

import (
	"os"
	"path/filepath"
	"strings"
)

func checkDuplicate(host *Host, path, added string) []Violation {
	if host == nil || strings.TrimSpace(added) == "" {
		return nil
	}
	symbols := extractSymbols(path, added)
	if len(symbols) == 0 {
		return nil
	}
	root := strings.TrimSpace(host.Root)
	if root == "" {
		return nil
	}
	rel := normalizePath(path)
	dir := filepath.Dir(rel)
	files := listSiblingSourceFiles(root, dir, rel)
	var out []Violation
	for _, sym := range symbols {
		if hit := findSymbolInFiles(root, files, sym); hit != "" {
			out = append(out, Violation{
				Rule:     RuleDuplicate,
				Severity: SeverityBlock,
				Message:  "duplicate implementation: symbol `" + sym + "` already exists in `" + hit + "`",
				Hint:     "Extend or call the existing `" + sym + "` instead of redefining it.",
			})
		}
	}
	return out
}

func listSiblingSourceFiles(root, dir, skip string) []string {
	absDir := filepath.Join(root, filepath.FromSlash(dir))
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".go" && ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".jsx" {
			continue
		}
		rel := normalizePath(filepath.Join(dir, name))
		if rel == skip {
			continue
		}
		out = append(out, rel)
	}
	return out
}

func findSymbolInFiles(root string, files []string, symbol string) string {
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		for _, sym := range extractSymbols(rel, string(data)) {
			if sym == symbol {
				return rel
			}
		}
	}
	return ""
}
