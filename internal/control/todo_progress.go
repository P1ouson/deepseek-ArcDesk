package control

import (
	"encoding/json"
	"fmt"
	"strings"
)

type todoProgressItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm,omitempty"`
	Level      int    `json:"level,omitempty"`
}

// FormatTodoContextBlock turns todo_write args JSON into a compact task-list block
// injected into user turns so the model can see the right-dock checklist.
func FormatTodoContextBlock(argsJSON string) string {
	argsJSON = strings.TrimSpace(argsJSON)
	if argsJSON == "" {
		return ""
	}
	var payload struct {
		Todos []todoProgressItem `json:"todos"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &payload); err != nil || len(payload.Todos) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<task-list>\n")
	for i, item := range payload.Todos {
		label := strings.TrimSpace(item.Content)
		if item.Status == "in_progress" && strings.TrimSpace(item.ActiveForm) != "" {
			label = strings.TrimSpace(item.ActiveForm)
		}
		indent := ""
		if item.Level > 0 {
			indent = strings.Repeat("  ", item.Level)
		}
		b.WriteString(fmt.Sprintf("%d. %s%s [%s]\n", i+1, indent, label, item.Status))
	}
	b.WriteString("</task-list>")
	return b.String()
}
