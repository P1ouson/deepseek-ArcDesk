package constraint

import (
	"os"
	"path/filepath"
	"strings"
)

var reuseFileNames = map[string]struct{}{
	"util.go": {}, "utils.go": {}, "helper.go": {}, "helpers.go": {},
	"common.go": {}, "shared.go": {},
	"util.ts": {}, "utils.ts": {}, "helper.ts": {}, "helpers.ts": {},
	"common.ts": {}, "shared.ts": {},
}

func checkReuse(host *Host, path, added string, kind string) []Violation {
	if host == nil {
		return nil
	}
	root := strings.TrimSpace(host.Root)
	if root == "" {
		return nil
	}
	rel := normalizePath(path)
	base := strings.ToLower(filepath.Base(rel))
	var out []Violation

	if kind == "create" {
		if _, ok := reuseFileNames[base]; ok {
			if alt := findExistingUtilityModule(root); alt != "" {
				out = append(out, Violation{
					Rule:     RuleReuse,
					Severity: SeverityWarn,
					Message:  "new utility file `" + rel + "` overlaps an existing helper module",
					Hint:     "Prefer reusing logic from `" + alt + "` instead of adding another utility module.",
				})
			}
		}
	}

	symbols := extractSymbols(rel, added)
	for _, sym := range symbols {
		if !looksLikeGenericHelper(sym) {
			continue
		}
		if hit := findSimilarSymbolElsewhere(host, rel, sym); hit != "" {
			out = append(out, Violation{
				Rule:     RuleReuse,
				Severity: SeverityWarn,
				Message:  "symbol `" + sym + "` resembles existing helper `" + hit + "`",
				Hint:     "Reuse `" + hit + "` or extend it instead of introducing parallel logic.",
			})
		}
	}
	return out
}

func looksLikeGenericHelper(name string) bool {
	lower := strings.ToLower(name)
	for _, stem := range []string{"helper", "util", "format", "parse", "validate", "normalize", "convert"} {
		if strings.Contains(lower, stem) {
			return true
		}
	}
	return false
}

func findExistingUtilityModule(root string) string {
	candidates := []string{
		"internal/util",
		"internal/common",
		"desktop/frontend/src/lib",
	}
	for _, dir := range candidates {
		abs := filepath.Join(root, filepath.FromSlash(dir))
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			continue
		}
		return dir
	}
	return ""
}

func findSimilarSymbolElsewhere(host *Host, skipPath, symbol string) string {
	root := strings.TrimSpace(host.Root)
	dir := filepath.Dir(normalizePath(skipPath))
	files := listSiblingSourceFiles(root, dir, skipPath)
	return findSymbolInFiles(root, files, symbol)
}
