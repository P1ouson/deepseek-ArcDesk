package verification

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/instruction"
	"arcdesk/internal/tool"
)

// RegisterTools adds verification_plan and verification_status tools.
func RegisterTools(reg *tool.Registry, plan Plan) {
	if reg == nil || len(plan.Checks) == 0 {
		return
	}
	reg.Add(verifyStatusTool{plan: plan})
	reg.Add(verifyPlanTool{plan: plan})
}

type verifyStatusTool struct{ plan Plan }

func (verifyStatusTool) Name() string { return "verification_status" }
func (verifyStatusTool) Description() string {
	return "Summarize configured post-write verification checks (build, unit, e2e) and retry policy."
}
func (verifyStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (verifyStatusTool) ReadOnly() bool { return true }
func (t verifyStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	counts := map[Category]int{}
	for _, c := range t.plan.Checks {
		counts[c.Category]++
	}
	b, _ := json.Marshal(t.plan)
	return fmt.Sprintf("Verification: %d checks (build=%d unit=%d e2e=%d custom=%d), max_retries=%d on_failure=%s\n%s",
		len(t.plan.Checks),
		counts[CategoryBuild], counts[CategoryUnit], counts[CategoryE2E], counts[CategoryCustom],
		t.plan.Policy.MaxRetries, t.plan.Policy.OnFailure, string(b)), nil
}

type verifyPlanTool struct{ plan Plan }

func (verifyPlanTool) Name() string { return "verification_plan" }
func (verifyPlanTool) Description() string {
	return "List every verification command the host requires after file writes, grouped by category."
}
func (verifyPlanTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (verifyPlanTool) ReadOnly() bool { return true }
func (t verifyPlanTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	if len(t.plan.Checks) == 0 {
		return "No verification checks configured.", nil
	}
	var b strings.Builder
	for _, cat := range []Category{CategoryBuild, CategoryUnit, CategoryE2E, CategoryCustom} {
		var lines []string
		for _, c := range t.plan.Checks {
			if c.Category != cat {
				continue
			}
			src := c.SourcePath
			if src == "" {
				src = "config"
			}
			lines = append(lines, fmt.Sprintf("- %s (from %s)", c.Command, src))
		}
		if len(lines) == 0 {
			continue
		}
		b.WriteString("## ")
		b.WriteString(string(cat))
		b.WriteByte('\n')
		for _, ln := range lines {
			b.WriteString(ln)
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String()), nil
}

// ChecksFromInstructions converts instruction checks to Plan checks.
func ChecksFromInstructions(checks []instruction.VerifyCheck) []Check {
	out := make([]Check, 0, len(checks))
	for _, c := range checks {
		cat := categoryOf(c.Category)
		if cat == CategoryCustom && c.SourcePath != discoverSource {
			// host/config checks without explicit category stay custom
		} else if c.Category == "" {
			cat = CategoryCustom
		}
		out = append(out, Check{
			Command:    c.Command,
			Category:   cat,
			SourcePath: c.SourcePath,
			Line:       c.Line,
		})
	}
	return out
}
