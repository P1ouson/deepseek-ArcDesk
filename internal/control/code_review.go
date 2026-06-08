package control

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const codeReviewTimeout = 15 * time.Minute

// CodeReviewResult is the outcome of a direct subagent code review.
type CodeReviewResult struct {
	Text string
	Err  string
}

// BuildCodeReviewTask formats the subagent task from review mode, scope, and paths.
// Mirrors desktop/frontend/src/lib/codeReview.ts buildCodeReviewPrompt.
func BuildCodeReviewTask(mode, scope string, paths []string) string {
	var scopeHint string
	switch scope {
	case "session":
		scopeHint = "Prioritize files changed in the current agent session."
	case "git":
		scopeHint = "Prioritize git working-tree / branch diff changes."
	case "both":
		scopeHint = "Focus on files changed both in-session and in git."
	default:
		scopeHint = "Review all pending session and git changes."
	}

	var b strings.Builder
	b.WriteString(scopeHint)
	if len(paths) > 0 {
		fmt.Fprintf(&b, "\n\nChanged paths (%d):", len(paths))
		for _, p := range paths {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			fmt.Fprintf(&b, "\n- %s", p)
		}
	} else {
		b.WriteString("\n\nDiscover changed files via git status / session checkpoints.")
	}

	b.WriteString(`

Requirements:
- Start with a one-sentence verdict (ship / minor fixes / blocking).
- Then bullet findings with file:line references where possible.
- Group by severity when there are multiple items.`)

	if mode == "security" {
		b.WriteString("\n- Focus on exploitable security issues only.")
	}

	return strings.TrimSpace(b.String())
}

// RunCodeReview invokes the review or security_review subagent tool directly,
// without routing through the main model turn.
func (c *Controller) RunCodeReview(mode, scope string, paths []string) CodeReviewResult {
	if c == nil {
		return CodeReviewResult{Err: "no active session"}
	}
	if c.Running() {
		return CodeReviewResult{Err: "agent is busy"}
	}
	if c.reg == nil {
		return CodeReviewResult{Err: "tool registry unavailable"}
	}

	toolName := "review"
	if mode == "security" {
		toolName = "security_review"
	}
	tl, ok := c.reg.Get(toolName)
	if !ok {
		return CodeReviewResult{Err: fmt.Sprintf("%q tool is not available (skill may be disabled)", toolName)}
	}

	task := BuildCodeReviewTask(mode, scope, paths)
	if strings.TrimSpace(task) == "" {
		return CodeReviewResult{Err: "empty review task"}
	}
	args, err := json.Marshal(map[string]string{"task": task})
	if err != nil {
		return CodeReviewResult{Err: err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), codeReviewTimeout)
	defer cancel()

	out, err := tl.Execute(ctx, args)
	if err != nil {
		return CodeReviewResult{Err: err.Error()}
	}
	text := strings.TrimSpace(out)
	if text == "" {
		return CodeReviewResult{Err: "review finished without producing a report"}
	}
	return CodeReviewResult{Text: text}
}
