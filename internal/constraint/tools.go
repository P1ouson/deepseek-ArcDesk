package constraint

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds constraint_status and constraint_check.
func RegisterTools(reg *tool.Registry, eng *Engine) {
	if reg == nil || eng == nil {
		return
	}
	reg.Add(constraintStatusTool{eng: eng})
	reg.Add(constraintCheckTool{eng: eng})
}

type constraintStatusTool struct{ eng *Engine }

func (constraintStatusTool) Name() string { return "constraint_status" }
func (constraintStatusTool) Description() string {
	return "Report constraint system health: enabled rules, check counters, and the last violation snapshot."
}
func (constraintStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (constraintStatusTool) ReadOnly() bool { return true }

func (t constraintStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	checks, blocks, warnings := t.eng.Stats()
	last := t.eng.LastResult()
	b, _ := json.Marshal(last)
	lines := []string{
		fmt.Sprintf("Constraint system: checks=%d blocks=%d warnings=%d", checks, blocks, warnings),
		"Rules: duplicate(block), reuse(warn), fake_ui(block), architecture(block)",
	}
	if len(last.Violations) > 0 {
		lines = append(lines, "Last violations:")
		for _, v := range last.Violations {
			lines = append(lines, fmt.Sprintf("- [%s/%s] %s", v.Rule, v.Severity, v.Message))
		}
	}
	return strings.Join(lines, "\n") + "\n" + string(b), nil
}

type constraintCheckTool struct{ eng *Engine }

func (constraintCheckTool) Name() string { return "constraint_check" }
func (constraintCheckTool) Description() string {
	return "Dry-run constraint checks for a path and optional old/new content before writing."
}
func (constraintCheckTool) Schema() json.RawMessage {
	return json.RawMessage(`{
  "type":"object",
  "required":["path"],
  "properties":{
    "path":{"type":"string"},
    "old_text":{"type":"string"},
    "new_text":{"type":"string"}
  }
}`)
}
func (constraintCheckTool) ReadOnly() bool { return true }

func (t constraintCheckTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	_ = ctx
	var in struct {
		Path    string `json:"path"`
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", err
	}
	if strings.TrimSpace(in.Path) == "" {
		return "", fmt.Errorf("path is required")
	}
	res := t.eng.CheckPath(in.Path, in.OldText, in.NewText)
	b, _ := json.Marshal(res)
	msg := "Constraint check: blocked=" + fmt.Sprintf("%v", res.Blocked)
	if res.Blocked {
		msg += " — " + res.FormatBlockMessage()
	}
	if hint := res.FormatWarnHint(); hint != "" {
		msg += "\n" + hint
	}
	return msg + "\n" + string(b), nil
}
