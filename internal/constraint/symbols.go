package constraint

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	goFuncRE     = regexp.MustCompile(`(?m)^func\s+(?:\([^)]*\)\s+)?(\w+)\s*\(`)
	tsExportFnRE = regexp.MustCompile(`(?m)^export\s+(?:async\s+)?function\s+(\w+)`)
	tsExportVarRE = regexp.MustCompile(`(?m)^export\s+(?:const|let|var)\s+(\w+)`)
	tsFnRE       = regexp.MustCompile(`(?m)^function\s+(\w+)`)
	tsHookRE     = regexp.MustCompile(`(?m)^export\s+function\s+(use[A-Z]\w*)`)
)

func addedLines(oldText, newText string) string {
	if oldText == newText {
		return ""
	}
	oldSet := map[string]struct{}{}
	for _, line := range strings.Split(oldText, "\n") {
		oldSet[line] = struct{}{}
	}
	var added []string
	for _, line := range strings.Split(newText, "\n") {
		if _, ok := oldSet[line]; !ok {
			added = append(added, line)
		}
	}
	return strings.Join(added, "\n")
}

func extractSymbols(path, text string) []string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return matchAll(goFuncRE, text)
	case ".ts", ".tsx", ".js", ".jsx":
		var out []string
		out = append(out, matchAll(tsExportFnRE, text)...)
		out = append(out, matchAll(tsExportVarRE, text)...)
		out = append(out, matchAll(tsFnRE, text)...)
		out = append(out, matchAll(tsHookRE, text)...)
		return dedupeStrings(out)
	default:
		return nil
	}
}

func matchAll(re *regexp.Regexp, text string) []string {
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 && m[1] != "" {
			out = append(out, m[1])
		}
	}
	return out
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := in[:0]
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func normalizePath(path string) string {
	return strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
}

func isFrontendPath(path string) bool {
	path = normalizePath(path)
	return strings.HasPrefix(path, "desktop/frontend/")
}

func isFrontendComponentPath(path string) bool {
	path = normalizePath(path)
	return strings.HasPrefix(path, "desktop/frontend/src/components/")
}

func isStylesheetPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".css" || ext == ".scss" || ext == ".sass"
}

func isGoInternalPath(path string) bool {
	path = normalizePath(path)
	return strings.HasPrefix(path, "internal/")
}

func isDesktopGoPath(path string) bool {
	path = normalizePath(path)
	return strings.HasPrefix(path, "desktop/") && strings.HasSuffix(path, ".go")
}
