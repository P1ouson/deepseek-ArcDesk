package planner

import (
	"strings"
)

// Phase is one top-level stage of a plan with optional nested steps.
type Phase struct {
	Title string   `json:"title"`
	Steps []string `json:"steps,omitempty"`
}

// ParsePhases splits markdown plan text into ordered phases.
// items and numbered headings become phases; indented sub-items become steps.
func ParsePhases(plan string) []Phase {
	plan = strings.TrimSpace(plan)
	if plan == "" {
		return nil
	}
	var phases []Phase
	var cur *Phase
	flush := func() {
		if cur != nil && strings.TrimSpace(cur.Title) != "" {
			phases = append(phases, *cur)
		}
		cur = nil
	}
	for _, raw := range strings.Split(plan, "\n") {
		item, level, ok := listItem(raw)
		if !ok {
			continue
		}
		if level == 0 {
			flush()
			cur = &Phase{Title: item}
			continue
		}
		if cur == nil {
			cur = &Phase{Title: item}
			continue
		}
		cur.Steps = append(cur.Steps, item)
	}
	flush()
	if len(phases) > 12 {
		phases = phases[:12]
	}
	return phases
}

func listItem(line string) (content string, level int, ok bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return "", 0, false
	}
	indent := 0
	for _, c := range line[:len(line)-len(trimmed)] {
		if c == '\t' {
			indent += 4
		} else {
			indent++
		}
	}
	s := trimmed
	heading := false
	if h := strings.TrimLeft(s, "#"); h != s && strings.HasPrefix(h, " ") {
		heading = true
		s = strings.TrimSpace(h)
	}
	switch {
	case strings.HasPrefix(s, "- "), strings.HasPrefix(s, "* "), strings.HasPrefix(s, "+ "):
		s = s[2:]
	default:
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == 0 || i+1 >= len(s) || (s[i] != '.' && s[i] != ')') || s[i+1] != ' ' {
			return "", 0, false
		}
		s = s[i+2:]
	}
	s = strings.TrimSpace(stripInlineMarkdown(s))
	if s == "" {
		return "", 0, false
	}
	if heading {
		return s, 0, true
	}
	if indent >= 2 {
		return s, 1, true
	}
	return s, 0, true
}

func stripInlineMarkdown(s string) string {
	s = strings.TrimSpace(s)
	for strings.HasPrefix(s, "**") && strings.HasSuffix(s, "**") && len(s) > 4 {
		s = strings.TrimSpace(s[2 : len(s)-2])
	}
	for strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`") && len(s) > 2 {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}
