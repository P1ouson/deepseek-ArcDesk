package guardian

import (
	"path/filepath"
	"regexp"
	"strings"

	"arcdesk/internal/archrag"
)

var backtickPathRE = regexp.MustCompile("`([^`]+)`")

// SpecRule is one enforceable bullet from an architecture document.
type SpecRule struct {
	Doc     string `json:"doc"`
	Line    int    `json:"line,omitempty"`
	Text    string `json:"text"`
	Paths   []string `json:"paths,omitempty"`
	Kind    string `json:"kind"` // prefer | forbid | require
}

// CompileRules extracts bullets from Rules/Architecture sections in the index.
func CompileRules(idx *archrag.Index) []SpecRule {
	if idx == nil {
		return nil
	}
	var out []SpecRule
	for _, s := range idx.Sections() {
		if !isRulesSection(s.Heading) {
			continue
		}
		for i, line := range strings.Split(s.Body, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "*") {
				continue
			}
			text := strings.TrimLeft(strings.TrimLeft(line, "-"), "* ")
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			out = append(out, SpecRule{
				Doc:   s.Doc,
				Line:  s.Line + i + 1,
				Text:  text,
				Paths: extractPaths(text),
				Kind:  classifyRule(text),
			})
		}
	}
	return out
}

func isRulesSection(heading string) bool {
	h := strings.ToLower(strings.TrimSpace(heading))
	switch h {
	case "rules", "architecture", "constraints", "architecture rules":
		return true
	default:
		return strings.Contains(h, "rule")
	}
}

func classifyRule(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "prefer"):
		return "prefer"
	case strings.Contains(lower, "must not"), strings.Contains(lower, "do not"), strings.Contains(lower, "never"):
		return "forbid"
	case strings.Contains(lower, "must"), strings.Contains(lower, "required"):
		return "require"
	default:
		return "guidance"
	}
}

func extractPaths(text string) []string {
	matches := backtickPathRE.FindAllStringSubmatch(text, -1)
	var paths []string
	seen := map[string]bool{}
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		p := filepath.ToSlash(strings.TrimSpace(m[1]))
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}

func checkSpecRules(rules []SpecRule, path, added string) []Violation {
	if len(rules) == 0 || strings.TrimSpace(added) == "" {
		return nil
	}
	rel := normalizePath(path)
	var out []Violation
	for _, rule := range rules {
		if v, ok := evalSpecRule(rule, rel, added); ok {
			out = append(out, v)
		}
	}
	return out
}

func evalSpecRule(rule SpecRule, rel, added string) (Violation, bool) {
	lower := strings.ToLower(rule.Text)
	switch {
	case rule.Kind == "prefer" && strings.Contains(lower, "internal/"):
		if prefersInternal(rule) && isDesktopGoPath(rel) && looksLikeGoBusinessLogic(added) {
			return Violation{
				Rule:     "spec_prefer_internal",
				Severity: SeverityWarn,
				Message:  "SPEC prefers business logic under internal/, not `" + rel + "`",
				Hint:     rule.Text,
				Source:   rule.Doc,
			}, true
		}
	case rule.Kind == "forbid" && strings.Contains(lower, "mock"):
		if isFrontendPath(rel) && looksLikeMockUI(added) {
			return Violation{
				Rule:     "spec_no_mock_ui",
				Severity: SeverityBlock,
				Message:  "SPEC forbids hardcoded mock UI fixes in `" + rel + "`",
				Hint:     rule.Text,
				Source:   rule.Doc,
			}, true
		}
	case strings.Contains(lower, "wails bind") && strings.Contains(lower, "desktop/"):
		if goBindMethod.FindStringIndex(added) != nil && !isDesktopGoPath(rel) {
			return Violation{
				Rule:     "spec_wails_bind",
				Severity: SeverityBlock,
				Message:  "Wails bind methods must live under desktop/, not `" + rel + "`",
				Hint:     rule.Text,
				Source:   rule.Doc,
			}, true
		}
	}
	return Violation{}, false
}

func prefersInternal(rule SpecRule) bool {
	for _, p := range rule.Paths {
		if strings.HasPrefix(p, "internal/") {
			return true
		}
	}
	return strings.Contains(strings.ToLower(rule.Text), "internal/")
}

func looksLikeGoBusinessLogic(added string) bool {
	lower := strings.ToLower(added)
	if strings.Contains(lower, "func ") && !strings.Contains(lower, "func (") {
		return true
	}
	if strings.Contains(lower, "state ") || strings.Contains(lower, "return state") {
		return true
	}
	return false
}

func looksLikeMockUI(added string) bool {
	lower := strings.ToLower(added)
	if strings.Contains(lower, "mock") || strings.Contains(lower, "placeholder") || strings.Contains(lower, "lorem ipsum") {
		return true
	}
	if strings.Contains(lower, "hardcoded") && (strings.Contains(lower, "data") || strings.Contains(lower, "value")) {
		return true
	}
	return false
}
