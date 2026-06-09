package prompt

import (
	"strings"

	"arcdesk/internal/tool"
)

// AlignWithRegistry appends a short scope note when the composed system prompt
// could imply tools that are not registered for this session (e.g. desktop
// workspace sessions omit todo_write and background-job helpers).
func AlignWithRegistry(sys string, reg *tool.Registry) string {
	if reg == nil {
		return sys
	}
	var notes []string
	if !hasTool(reg, "todo_write") {
		notes = append(notes, "todo_write and complete_step are not registered in this session — track multi-step progress in your replies instead of calling them.")
	}
	if !hasTool(reg, "wait") {
		notes = append(notes, "Background job helpers (bash run_in_background, bash_output, wait, kill_shell) are not registered — run shell commands in the foreground and prefer read_file, grep, glob, and ls over shell for searching and reading files.")
	}
	if len(notes) == 0 {
		return sys
	}
	return strings.TrimSpace(sys) + "\n\n# Session tool scope\n\n" + strings.Join(notes, " ")
}

func hasTool(reg *tool.Registry, name string) bool {
	_, ok := reg.Get(name)
	return ok
}
