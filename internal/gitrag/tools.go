package gitrag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/tool"
)

// RegisterTools adds git-aware retrieval tools to reg.
func RegisterTools(reg *tool.Registry, repo *Repo) {
	if reg == nil || repo == nil {
		return
	}
	reg.Add(gitStatusTool{repo: repo})
	reg.Add(gitBlameTool{repo: repo})
	reg.Add(gitLogTool{repo: repo})
	reg.Add(gitShowTool{repo: repo})
	reg.Add(gitPRContextTool{repo: repo})
	reg.Add(gitIssueContextTool{repo: repo})
}

type gitStatusTool struct{ repo *Repo }

func (gitStatusTool) Name() string { return "git_status" }
func (gitStatusTool) Description() string {
	return "Git repository status for the workspace: toplevel path, branch, HEAD, detached state."
}
func (gitStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (gitStatusTool) ReadOnly() bool { return true }
func (t gitStatusTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	branch, head, detached, err := t.repo.Status(ctx)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"root": t.repo.Root, "workspace": t.repo.Work,
		"branch": branch, "head": head, "detached": detached,
		"gh_available": GHAvailable(),
	}
	b, _ := json.Marshal(payload)
	return fmt.Sprintf("Git repo at %s on %s (%s)\n%s", t.repo.Root, branch, head, string(b)), nil
}

type gitBlameTool struct{ repo *Repo }

func (gitBlameTool) Name() string { return "git_blame" }
func (gitBlameTool) Description() string {
	return "Line-level git blame for a repo-relative file. Use before editing unfamiliar code to see recent authors and commits."
}
func (gitBlameTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"start_line":{"type":"integer"},"end_line":{"type":"integer"},"limit":{"type":"integer"}},"required":["path"]}`)
}
func (gitBlameTool) ReadOnly() bool { return true }
func (t gitBlameTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Path      string `json:"path"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
		Limit     int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	lines, err := t.repo.Blame(ctx, p.Path, p.StartLine, p.EndLine, p.Limit)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(lines)
	if len(lines) == 0 {
		return "No blame lines.\n[]", nil
	}
	return fmt.Sprintf("%d blame line(s) for %s:\n%s", len(lines), p.Path, string(b)), nil
}

type gitLogTool struct{ repo *Repo }

func (gitLogTool) Name() string { return "git_log" }
func (gitLogTool) Description() string {
	return "Recent commit history for a file or the whole repository. Prefer this over raw bash git log."
}
func (gitLogTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Repo-relative file; omit for repo-wide history"},"limit":{"type":"integer"},"since":{"type":"string","description":"Git --since value, e.g. 2.weeks"}}}`)
}
func (gitLogTool) ReadOnly() bool { return true }
func (t gitLogTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Path  string `json:"path"`
		Limit int    `json:"limit"`
		Since string `json:"since"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &p)
	}
	commits, err := t.repo.Log(ctx, p.Path, p.Limit, p.Since)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(commits)
	scope := p.Path
	if scope == "" {
		scope = "(repository)"
	}
	if len(commits) == 0 {
		return fmt.Sprintf("No commits for %s.\n[]", scope), nil
	}
	return fmt.Sprintf("%d commit(s) for %s:\n%s", len(commits), scope, string(b)), nil
}

type gitShowTool struct{ repo *Repo }

func (gitShowTool) Name() string { return "git_show" }
func (gitShowTool) Description() string {
	return "Show a commit message and diff stat for a hash returned by git_log or git_blame."
}
func (gitShowTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"commit":{"type":"string"}},"required":["commit"]}`)
}
func (gitShowTool) ReadOnly() bool { return true }
func (t gitShowTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	return t.repo.ShowCommit(ctx, p.Commit)
}

type gitPRContextTool struct{ repo *Repo }

func (gitPRContextTool) Name() string { return "git_pr_context" }
func (gitPRContextTool) Description() string {
	return "GitHub pull request context for the current branch via gh CLI (requires gh auth). Falls back with a clear error when unavailable."
}
func (gitPRContextTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"list_open":{"type":"boolean","description":"When true, list open PRs instead of viewing the current branch PR"}}}`)
}
func (gitPRContextTool) ReadOnly() bool { return true }
func (t gitPRContextTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		ListOpen bool `json:"list_open"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &p)
	}
	if p.ListOpen {
		return t.repo.PRList(ctx, 10)
	}
	out, err := t.repo.PRContext(ctx)
	if err != nil {
		return "", err
	}
	return out, nil
}

type gitIssueContextTool struct{ repo *Repo }

func (gitIssueContextTool) Name() string { return "git_issue_context" }
func (gitIssueContextTool) Description() string {
	return "GitHub issue details via gh CLI (requires gh auth)."
}
func (gitIssueContextTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"number":{"type":"string"}},"required":["number"]}`)
}
func (gitIssueContextTool) ReadOnly() bool { return true }
func (t gitIssueContextTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Number string `json:"number"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	out, err := t.repo.IssueContext(ctx, strings.TrimSpace(p.Number))
	if err != nil {
		return "", err
	}
	return out, nil
}
