package guardian

import (
	"regexp"
	"strings"
)

var goBindMethod = regexp.MustCompile(`(?m)^func\s+\(\s*\w+\s+\*App\s*\)\s+(\w+)\s*\(`)

func normalizePath(path string) string {
	return strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
}

func isFrontendPath(path string) bool {
	path = normalizePath(path)
	return strings.HasPrefix(path, "desktop/frontend/")
}

func isDesktopGoPath(path string) bool {
	path = normalizePath(path)
	return strings.HasPrefix(path, "desktop/") && strings.HasSuffix(strings.ToLower(path), ".go")
}

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
