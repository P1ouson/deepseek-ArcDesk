package guardian

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds architecture_guardian_* tools.
func RegisterTools(reg *tool.Registry, g *Guardian) {
	if reg == nil || g == nil {
		return
	}
	reg.Add(guardianStatusTool{g: g})
	reg.Add(guardianRulesTool{g: g})
	reg.Add(guardianCheckTool{g: g})
}

type guardianStatusTool struct{ g *Guardian }

func (guardianStatusTool) Name() string { return "architecture_guardian_status" }
func (guardianStatusTool) Description() string {
	return "Report Architecture Guardian health: loaded SPEC rules, check counters, and last finding snapshot."
}
func (guardianStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (guardianStatusTool) ReadOnly() bool { return true }

func (t guardianStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	last := t.g.LastResult()
	b, _ := json.Marshal(last)
	lines := []string{
		t.g.SummaryLine(),
		"Tools: architecture_guardian_rules, architecture_guardian_check",
	}
	if len(last.Violations) > 0 {
		lines = append(lines, "Last violations:")
		for _, v := range last.Violations {
			lines = append(lines, fmt.Sprintf("- [%s/%s] %s", v.Rule, v.Severity, v.Message))
		}
	}
	return strings.Join(lines, "\n") + "\n" + string(b), nil
}

type guardianRulesTool struct{ g *Guardian }

func (guardianRulesTool) Name() string { return "architecture_guardian_rules" }
func (guardianRulesTool) Description() string {
	return "List SPEC/architecture rules compiled from indexed docs (Rules/Architecture sections)."
}
func (guardianRulesTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (guardianRulesTool) ReadOnly() bool { return true }

func (t guardianRulesTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	_ = ctx
	rules := t.g.Rules()
	b, _ := json.Marshal(rules)
	if len(rules) == 0 {
		return "No SPEC rules compiled from architecture docs.\n[]", nil
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("%d SPEC rule(s):", len(rules)))
	for _, r := range rules {
		lines = append(lines, fmt.Sprintf("- [%s] (%s) %s", r.Kind, r.Doc, r.Text))
	}
	return strings.Join(lines, "\n") + "\n" + string(b), nil
}

type guardianCheckTool struct{ g *Guardian }

func (guardianCheckTool) Name() string { return "architecture_guardian_check" }
func (guardianCheckTool) Description() string {
	return "Dry-run Architecture Guardian checks for a path and optional old/new content before writing."
}
func (guardianCheckTool) Schema() json.RawMessage {
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
func (guardianCheckTool) ReadOnly() bool { return true }

func (t guardianCheckTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
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
	res := t.g.CheckPath(in.Path, in.OldText, in.NewText)
	b, _ := json.Marshal(res)
	msg := "Architecture guardian check: blocked=" + fmt.Sprintf("%v", res.Blocked)
	if res.Blocked {
		msg += " — " + res.FormatBlockMessage()
	}
	if hint := res.FormatWarnHint(); hint != "" {
		msg += "\n" + hint
	}
	return msg + "\n" + string(b), nil
}
