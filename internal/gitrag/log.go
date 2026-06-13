package gitrag

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Commit is a simplified git log entry.
type Commit struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Email   string `json:"email"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

// Log returns recent commits touching path (or whole repo when path is empty).
func (r *Repo) Log(ctx context.Context, path string, limit int, since string) ([]Commit, error) {
	if r == nil {
		return nil, fmt.Errorf("nil repo")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	args := []string{"log", fmt.Sprintf("-n%d", limit),
		`--pretty=format:%H%x1f%an%x1f%ae%x1f%ad%x1f%s`, "--date=iso-strict"}
	since = strings.TrimSpace(since)
	if since != "" {
		args = append(args, "--since="+since)
	}
	if path = strings.TrimSpace(path); path != "" {
		rel, err := r.relPath(path)
		if err != nil {
			return nil, err
		}
		args = append(args, "--follow", "--", rel)
	}
	out, err := runGit(ctx, r.Root, args...)
	if err != nil {
		return nil, err
	}
	return parseLogRecords(out), nil
}

func parseLogRecords(out string) []Commit {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil
	}
	var res []Commit
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\x1f")
		if len(parts) < 5 {
			continue
		}
		res = append(res, Commit{
			Hash:    parts[0],
			Author:  parts[1],
			Email:   parts[2],
			Date:    parts[3],
			Subject: parts[4],
		})
	}
	return res
}

// ShowCommit returns the patch stat + message for a commit hash (short or full).
func (r *Repo) ShowCommit(ctx context.Context, hash string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("nil repo")
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return "", fmt.Errorf("commit hash is required")
	}
	out, err := runGit(ctx, r.Root, "show", "--stat", "--format=fuller", "--no-color", hash)
	if err != nil {
		return "", err
	}
	const maxBytes = 64 * 1024
	if len(out) > maxBytes {
		out = out[:maxBytes] + "\n… [truncated]"
	}
	return out, nil
}

// FileAuthors returns distinct authors who touched path (from recent log).
func (r *Repo) FileAuthors(ctx context.Context, path string, limit int) ([]string, error) {
	commits, err := r.Log(ctx, path, limit, "")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var authors []string
	for _, c := range commits {
		name := strings.TrimSpace(c.Author)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		authors = append(authors, name)
	}
	return authors, nil
}

// CommitCount returns approximate commit count for path (capped scan).
func (r *Repo) CommitCount(ctx context.Context, path string) (int, error) {
	rel := ""
	if strings.TrimSpace(path) != "" {
		var err error
		rel, err = r.relPath(path)
		if err != nil {
			return 0, err
		}
	}
	args := []string{"rev-list", "--count", "HEAD"}
	if rel != "" {
		args = append(args, "--", rel)
	}
	out, err := runGit(ctx, r.Root, args...)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, err
	}
	return n, nil
}
