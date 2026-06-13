package gitrag

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GHAvailable reports whether the gh CLI is on PATH.
func GHAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// PRContext returns gh pr view JSON/text for the current branch, or an error when unavailable.
func (r *Repo) PRContext(ctx context.Context) (string, error) {
	if r == nil {
		return "", fmt.Errorf("nil repo")
	}
	if !GHAvailable() {
		return "", fmt.Errorf("gh CLI not installed")
	}
	out, err := runGH(ctx, r.Root, "pr", "view", "--json", "number,title,body,url,state,author,baseRefName,headRefName")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// IssueContext returns gh issue view JSON for issue number.
func (r *Repo) IssueContext(ctx context.Context, number string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("nil repo")
	}
	number = strings.TrimSpace(number)
	if number == "" {
		return "", fmt.Errorf("issue number is required")
	}
	if !GHAvailable() {
		return "", fmt.Errorf("gh CLI not installed")
	}
	out, err := runGH(ctx, r.Root, "issue", "view", number, "--json", "number,title,body,url,state,author,labels,comments")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// PRList returns open PRs (limited) for quick context.
func (r *Repo) PRList(ctx context.Context, limit int) (string, error) {
	if r == nil {
		return "", fmt.Errorf("nil repo")
	}
	if !GHAvailable() {
		return "", fmt.Errorf("gh CLI not installed")
	}
	if limit <= 0 {
		limit = 10
	}
	out, err := runGH(ctx, r.Root, "pr", "list", "--state", "open", "--limit", fmt.Sprintf("%d", limit),
		"--json", "number,title,url,headRefName,author")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
