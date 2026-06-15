package control

import (
	"strings"
	"testing"
)

func TestFormatTodoContextBlock(t *testing.T) {
	args := `{"todos":[{"content":"Add parser","status":"in_progress"},{"content":"Add tests","status":"pending"}]}`
	got := FormatTodoContextBlock(args)
	if got == "" {
		t.Fatal("expected non-empty block")
	}
	for _, part := range []string{"<task-list>", "Add parser", "in_progress", "Add tests", "pending", "</task-list>"} {
		if !strings.Contains(got, part) {
			t.Fatalf("FormatTodoContextBlock = %q, want substring %q", got, part)
		}
	}
	if FormatTodoContextBlock("") != "" {
		t.Fatal("empty args should return empty block")
	}
}
