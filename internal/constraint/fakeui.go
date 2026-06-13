package constraint

import (
	"regexp"
	"strings"
)

var fakeUIPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bmock(?:Data|Response|Result)?\b`),
	regexp.MustCompile(`(?i)\bfake(?:Success|Data|Response)?\b`),
	regexp.MustCompile(`(?i)\bstub(?:Response|Data)?\b`),
	regexp.MustCompile(`(?i)//\s*TODO:\s*wire`),
	regexp.MustCompile(`(?i)set(?:Success|Loading)\(\s*true\s*\)\s*;\s*//\s*hack`),
	regexp.MustCompile(`(?i)success:\s*true\s*,?\s*//\s*(?:fake|mock|hack|temp)`),
	regexp.MustCompile(`(?i)display:\s*none\s*;?\s*//\s*hide`),
}

func checkFakeUI(path, oldText, newText, added string) []Violation {
	rel := normalizePath(path)
	if !isFrontendPath(rel) {
		return nil
	}
	var out []Violation

	if isStylesheetPath(rel) && strings.TrimSpace(added) != "" {
		out = append(out, Violation{
			Rule:     RuleFakeUI,
			Severity: SeverityBlock,
			Message:  "CSS-only edits cannot fix functional bugs in Wails UI",
			Hint:     "Wire the real handler/bridge/backend path instead of hiding symptoms with styles.",
		})
		return out
	}

	if !strings.HasSuffix(strings.ToLower(rel), ".tsx") &&
		!strings.HasSuffix(strings.ToLower(rel), ".ts") &&
		!strings.HasSuffix(strings.ToLower(rel), ".jsx") &&
		!strings.HasSuffix(strings.ToLower(rel), ".js") {
		return nil
	}

	for _, re := range fakeUIPatterns {
		if re.FindStringIndex(added) != nil {
			if !hasBridgeWiring(newText) {
				out = append(out, Violation{
					Rule:     RuleFakeUI,
					Severity: SeverityBlock,
					Message:  "fake UI fix detected: mock/stub/hardcoded success without bridge wiring",
					Hint:     "Call `app.*` through the bridge layer or fix the Go bind handler; do not hardcode success.",
				})
				break
			}
		}
	}

	if cssOnlyClassTweak(oldText, newText, added) && !hasBridgeWiring(newText) {
		out = append(out, Violation{
			Rule:     RuleFakeUI,
			Severity: SeverityBlock,
			Message:  "UI-only className/style tweak without backend wiring",
			Hint:     "Functional fixes must traverse UI → bridge → Go bind, not only adjust presentation.",
		})
	}
	return out
}

func hasBridgeWiring(text string) bool {
	text = strings.ToLower(text)
	return strings.Contains(text, "app.") ||
		strings.Contains(text, "bridge.ts") ||
		strings.Contains(text, "eventson(") ||
		strings.Contains(text, "window.go.main.app")
}

func cssOnlyClassTweak(oldText, newText, added string) bool {
	if strings.TrimSpace(added) == "" {
		return false
	}
	for _, line := range strings.Split(added, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "classname") || strings.Contains(lower, "style=") {
			continue
		}
		if strings.HasPrefix(lower, "//") || strings.HasPrefix(lower, "/*") {
			continue
		}
		return false
	}
	return oldText != newText
}
