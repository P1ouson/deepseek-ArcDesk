package gitrag

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"arcdesk/internal/proc"
)

const gitTimeout = 15 * time.Second

// Repo is a git toplevel bound to an ArcDesk workspace.
type Repo struct {
	Root string // git toplevel
	Work string // workspace root passed to Open
}

// Open resolves the git toplevel for workspaceRoot.
func Open(workspaceRoot string) (*Repo, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return nil, fmt.Errorf("empty workspace root")
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	root, err := gitTopLevel(ctx, workspaceRoot)
	if err != nil {
		return nil, err
	}
	return &Repo{Root: root, Work: workspaceRoot}, nil
}

// Status returns branch, head sha, and whether HEAD is detached.
func (r *Repo) Status(ctx context.Context) (branch, head string, detached bool, err error) {
	if r == nil {
		return "", "", false, fmt.Errorf("nil repo")
	}
	if branch, err = runGit(ctx, r.Root, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil {
		branch = strings.TrimSpace(branch)
	} else if short, err2 := runGit(ctx, r.Root, "rev-parse", "--short", "HEAD"); err2 == nil {
		branch = strings.TrimSpace(short)
		detached = true
	} else {
		return "", "", false, err
	}
	head, err = runGit(ctx, r.Root, "rev-parse", "HEAD")
	if err != nil {
		return branch, "", detached, err
	}
	head = strings.TrimSpace(head)
	return branch, head, detached, nil
}

func gitTopLevel(ctx context.Context, dir string) (string, error) {
	out, err := runGit(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	root := strings.TrimSpace(out)
	if root == "" {
		return "", fmt.Errorf("empty git toplevel")
	}
	return filepath.Clean(root), nil
}

func (r *Repo) relPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(path) {
		rel, err := filepath.Rel(r.Root, filepath.Clean(path))
		if err != nil {
			return "", err
		}
		path = rel
	}
	path = filepath.ToSlash(path)
	if strings.HasPrefix(path, "../") || path == ".." {
		return "", fmt.Errorf("path %q is outside the git repository", path)
	}
	return path, nil
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	proc.HideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("%s: %s", strings.TrimSpace(err.Error()), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}

func runGH(ctx context.Context, dir string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	proc.HideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("%s: %s", strings.TrimSpace(err.Error()), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}
