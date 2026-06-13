package dependency

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// ParseGoImports extracts import paths from a Go source file (parser fallback).
// Comment-only imports are ignored because go/parser skips comment text in imports.
func ParseGoImports(filePath string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, spec := range f.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out, nil
}

// checkGoFileSyntax reports syntax errors in a Go source file. Unlike ParseGoImports
// (ImportsOnly), this parses the full file body so broken non-import syntax is detected.
func checkGoFileSyntax(filePath string) error {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, filePath, nil, parser.SkipObjectResolution)
	return err
}

// inferImportPathFromDir maps a directory under module root to an import path.
func inferImportPathFromDir(modulePath, root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return ""
	}
	rel = normalizeSlash(rel)
	if rel == "." {
		return modulePath
	}
	return modulePath + "/" + rel
}
