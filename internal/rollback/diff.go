package rollback

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"arcdesk/internal/checkpoint"
	"arcdesk/internal/diff"
)

// BuildReport computes unified diffs from the current workspace back to the
// checkpoint state described by plan (what RestoreCode would apply).
func BuildReport(root string, plan checkpoint.RestorePlan) Report {
	report := Report{
		Turn:   plan.FromTurn,
		Prompt: strings.TrimSpace(plan.Prompt),
	}
	var modified, created, deleted int
	for _, target := range plan.Targets {
		fr, ok := buildFileRevert(root, target)
		if !ok {
			continue
		}
		report.Files = append(report.Files, fr)
		switch fr.Action {
		case "create":
			created++
		case "delete":
			deleted++
		default:
			modified++
		}
	}
	report.Summary = fmt.Sprintf("%d file(s): %d modified, %d created, %d deleted",
		len(report.Files), modified, created, deleted)
	return report
}

func buildFileRevert(root string, target checkpoint.RestoreTarget) (FileRevert, bool) {
	current, exists, err := readWorkspaceFile(root, target.Path)
	if err != nil {
		return FileRevert{}, false
	}

	var oldText, newText string
	action := "modify"
	switch {
	case target.Content == nil:
		if !exists {
			return FileRevert{}, false
		}
		oldText = current
		newText = ""
		action = "create"
	case !exists:
		oldText = ""
		newText = *target.Content
		action = "delete"
	default:
		oldText = current
		newText = *target.Content
		if oldText == newText {
			return FileRevert{}, false
		}
	}

	kind := diff.Modify
	if action == "create" {
		kind = diff.Delete
	} else if action == "delete" {
		kind = diff.Create
	}
	change := diff.Build(target.Path, oldText, newText, kind)
	return FileRevert{
		Path:    target.Path,
		Action:  action,
		Added:   change.Added,
		Removed: change.Removed,
		Diff:    change.Diff,
	}, true
}

func readWorkspaceFile(root, rel string) (content string, exists bool, err error) {
	rel = strings.ReplaceAll(strings.TrimSpace(rel), "\\", "/")
	if rel == "" {
		return "", false, fmt.Errorf("empty path")
	}
	abs := rel
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, rel)
	}
	abs = filepath.Clean(abs)
	if root != "" {
		r := filepath.Clean(root)
		if abs != r && !strings.HasPrefix(abs, r+string(os.PathSeparator)) {
			return "", false, fmt.Errorf("path %q escapes workspace", rel)
		}
	}
	b, err := os.ReadFile(abs)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(b), true, nil
}
