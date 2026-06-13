package constraint

import (
	"regexp"
	"strings"
)

var (
	goImportRE   = regexp.MustCompile(`(?m)^import\s+(?:\([^)]+\)|"[^"]+")`)
	goBindMethod = regexp.MustCompile(`(?m)^func\s+\(\s*\w+\s+\*App\s*\)\s+(\w+)\s*\(`)
	tsWailsImport = regexp.MustCompile(`(?m)(?:import|from)\s+['"][^'"]*wailsjs/go`)
)

func checkArchitecture(path, added string) []Violation {
	if strings.TrimSpace(added) == "" {
		return nil
	}
	rel := normalizePath(path)
	var out []Violation

	if strings.HasSuffix(strings.ToLower(rel), ".go") {
		out = append(out, checkGoArchitecture(rel, added)...)
	}
	if strings.HasSuffix(strings.ToLower(rel), ".ts") ||
		strings.HasSuffix(strings.ToLower(rel), ".tsx") ||
		strings.HasSuffix(strings.ToLower(rel), ".js") ||
		strings.HasSuffix(strings.ToLower(rel), ".jsx") {
		out = append(out, checkTSArchitecture(rel, added)...)
	}
	return out
}

func checkGoArchitecture(rel, added string) []Violation {
	var out []Violation
	for _, block := range goImportRE.FindAllString(added, -1) {
		if isGoInternalPath(rel) && strings.Contains(block, "desktop/frontend") {
			out = append(out, Violation{
				Rule:     RuleArch,
				Severity: SeverityBlock,
				Message:  "internal packages must not import desktop frontend code",
				Hint:     "Keep UI in `desktop/frontend` and expose behavior through Wails bind methods.",
			})
		}
	}
	if goBindMethod.FindStringIndex(added) != nil && !isDesktopGoPath(rel) {
		out = append(out, Violation{
			Rule:     RuleArch,
			Severity: SeverityBlock,
			Message:  "Wails bind methods must live under `desktop/*.go`, not `" + rel + "`",
			Hint:     "Add `func (a *App) Method(...)` in the desktop app package and call it from the bridge.",
		})
	}
	return out
}

func checkTSArchitecture(rel, added string) []Violation {
	if !isFrontendComponentPath(rel) {
		return nil
	}
	if tsWailsImport.FindStringIndex(added) != nil {
		return []Violation{{
			Rule:     RuleArch,
			Severity: SeverityBlock,
			Message:  "components must not import `wailsjs/go` directly",
			Hint:     "Route calls through `desktop/frontend/src/lib/bridge.ts` or a hook that wraps bridge calls.",
		}}
	}
	return nil
}
