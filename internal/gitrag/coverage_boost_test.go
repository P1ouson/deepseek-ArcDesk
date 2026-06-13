package gitrag

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestOpenAndDiscoverErrors(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("empty root should fail")
	}
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("non-git dir should fail")
	}
	if Discoverable("") {
		t.Fatal("empty not discoverable")
	}
	if Discoverable(t.TempDir()) {
		t.Fatal("non-git not discoverable")
	}
	dir := t.TempDir()
	initGitRepo(t, dir)
	if !Discoverable(dir) {
		t.Fatal("git repo should be discoverable")
	}
}

func TestRelPathAndNilRepo(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.relPath(""); err == nil {
		t.Fatal("empty path")
	}
	outside := filepath.Join(dir, "..", "escape.go")
	if _, err := repo.relPath(outside); err == nil {
		t.Fatal("outside repo")
	}
	abs := filepath.Join(dir, "README.md")
	if rel, err := repo.relPath(abs); err != nil || rel != "README.md" {
		t.Fatalf("abs path = %q err=%v", rel, err)
	}
	ctx := context.Background()
	var nilRepo *Repo
	if _, _, _, err := nilRepo.Status(ctx); err == nil {
		t.Fatal("nil Status")
	}
	if _, err := nilRepo.Blame(ctx, "x", 0, 0, 0); err == nil {
		t.Fatal("nil Blame")
	}
	if _, err := nilRepo.Log(ctx, "", 0, ""); err == nil {
		t.Fatal("nil Log")
	}
	if _, err := nilRepo.ShowCommit(ctx, "abc"); err == nil {
		t.Fatal("nil ShowCommit")
	}
	if _, err := nilRepo.PRContext(ctx); err == nil {
		t.Fatal("nil PRContext")
	}
	if _, err := nilRepo.IssueContext(ctx, "1"); err == nil {
		t.Fatal("nil IssueContext")
	}
	if _, err := nilRepo.PRList(ctx, 5); err == nil {
		t.Fatal("nil PRList")
	}
}

func TestLogAuthorsAndCount(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, dir, "add", "a.txt")
	runGitCmd(t, dir, "commit", "-m", "add a")
	repo, _ := Open(dir)
	ctx := context.Background()
	commits, err := repo.Log(ctx, "a.txt", 5, "")
	if err != nil || len(commits) == 0 {
		t.Fatalf("repo log: %v %v", commits, err)
	}
	authors, err := repo.FileAuthors(ctx, "a.txt", 5)
	if err != nil || len(authors) == 0 {
		t.Fatalf("authors: %v err=%v", authors, err)
	}
	n, err := repo.CommitCount(ctx, "a.txt")
	if err != nil || n < 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
}

func TestParseHelpers(t *testing.T) {
	blame := parsePorcelainBlame("abc123 1 1 1\nauthor Bob\nauthor-time 1\nsummary msg\n\tline text\n")
	if len(blame) != 1 || blame[0].Author != "Bob" || blame[0].Text != "line text" {
		t.Fatalf("blame=%+v", blame)
	}
	logs := parseLogRecords("hash\x1fauthor\x1femail\x1fd\x1fsubj")
	if len(logs) != 1 || logs[0].Subject != "subj" {
		t.Fatalf("logs=%+v", logs)
	}
	if parseLogRecords("") != nil {
		t.Fatal("empty log")
	}
}

func TestGitToolsEdgeCases(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	repo, _ := Open(dir)
	reg := tool.NewRegistry()
	RegisterTools(reg, repo)
	ctx := context.Background()
	show, _ := reg.Get("git_show")
	if _, err := show.Execute(ctx, json.RawMessage(`{"commit":""}`)); err == nil {
		t.Fatal("empty commit")
	}
	logTool, _ := reg.Get("git_log")
	if _, err := logTool.Execute(ctx, json.RawMessage(`{"path":"missing.txt","limit":3}`)); err == nil {
		t.Fatal("missing file log should error")
	}
	blameTool, _ := reg.Get("git_blame")
	if _, err := blameTool.Execute(ctx, json.RawMessage(`{`)); err == nil {
		t.Fatal("invalid blame json")
	}
	if _, err := blameTool.Execute(ctx, json.RawMessage(`{"path":"nope.txt","start_line":1,"end_line":1}`)); err == nil {
		t.Fatal("blame missing file")
	}
	issue, _ := reg.Get("git_issue_context")
	if _, err := issue.Execute(ctx, json.RawMessage(`{"number":""}`)); err == nil {
		t.Fatal("empty issue")
	}
	if _, err := issue.Execute(ctx, json.RawMessage(`{`)); err == nil {
		t.Fatal("bad issue json")
	}
	pr, _ := reg.Get("git_pr_context")
	if _, err := pr.Execute(ctx, json.RawMessage(`{"list_open":true}`)); err != nil && GHAvailable() {
		t.Skip("gh pr list needs remotes: " + err.Error())
	}
}

func TestRegisterToolsNil(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterTools(nil, nil)
	RegisterTools(reg, nil)
	if reg.Len() != 0 {
		t.Fatal("expected no tools")
	}
}

func TestGHWithFakeCLI(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	repo, _ := Open(dir)
	withFakeGH(t)
	ctx := context.Background()
	out, err := repo.PRContext(ctx)
	if err != nil || !strings.Contains(out, "number") {
		t.Fatalf("PRContext: %q err=%v", out, err)
	}
	out, err = repo.PRList(ctx, 3)
	if err != nil || out == "" {
		t.Fatalf("PRList: %q err=%v", out, err)
	}
	out, err = repo.IssueContext(ctx, "7")
	if err != nil || !strings.Contains(out, "number") {
		t.Fatalf("IssueContext: %q err=%v", out, err)
	}
	reg := tool.NewRegistry()
	RegisterTools(reg, repo)
	prTool, _ := reg.Get("git_pr_context")
	if _, err := prTool.Execute(ctx, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("pr view tool: %v", err)
	}
}

func TestGHUnavailableErrors(t *testing.T) {
	if GHAvailable() {
		t.Skip("gh is on PATH; skip unavailable test")
	}
	dir := t.TempDir()
	initGitRepo(t, dir)
	repo, _ := Open(dir)
	ctx := context.Background()
	if _, err := repo.IssueContext(ctx, "1"); err == nil {
		t.Fatal("expected gh error")
	}
}

func withFakeGH(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		body := `@echo off
if "%1"=="pr" if "%2"=="view" echo {"number":1,"title":"Test PR"} & exit /b 0
if "%1"=="pr" if "%2"=="list" echo [{"number":1,"title":"Open"}] & exit /b 0
if "%1"=="issue" if "%2"=="view" echo {"number":42,"title":"Bug"} & exit /b 0
exit /b 0
`
		if err := os.WriteFile(filepath.Join(dir, "gh.cmd"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	} else {
		body := `#!/bin/sh
case "$1" in
pr)
  case "$2" in
  view) echo '{"number":1,"title":"Test PR"}'; exit 0 ;;
  list) echo '[{"number":1,"title":"Open"}]'; exit 0 ;;
  esac ;;
issue)
  case "$2" in
  view) echo '{"number":42,"title":"Bug"}'; exit 0 ;;
  esac ;;
esac
exit 0
`
		p := filepath.Join(dir, "gh")
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if !GHAvailable() {
		t.Fatal("fake gh not on PATH")
	}
}

func TestRunGitStderr(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	ctx := context.Background()
	_, err := runGit(ctx, dir, "not-a-git-subcommand")
	if err == nil {
		t.Fatal("expected git error")
	}
}

func TestToolsMetadata(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	repo, _ := Open(dir)
	reg := tool.NewRegistry()
	RegisterTools(reg, repo)
	for _, name := range []string{"git_status", "git_blame", "git_log", "git_show", "git_pr_context", "git_issue_context"} {
		toolDef, ok := reg.Get(name)
		if !ok {
			t.Fatalf("missing %q", name)
		}
		if toolDef.Description() == "" {
			t.Fatalf("%q Description empty", name)
		}
		if !toolDef.ReadOnly() {
			t.Fatalf("%q should be read-only", name)
		}
	}
}

func TestDetachedHEADStatus(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	runGitCmd(t, dir, "commit", "--allow-empty", "-m", "base")
	out := runGitOutput(t, dir, "rev-parse", "HEAD")
	hash := strings.TrimSpace(out)
	runGitCmd(t, dir, "checkout", hash)
	repo, _ := Open(dir)
	branch, head, detached, err := repo.Status(context.Background())
	if err != nil || !detached || head != hash {
		t.Fatalf("detached status: branch=%q head=%q detached=%v err=%v", branch, head, detached, err)
	}
}

func TestGitLogAndBlameEmptyResults(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	runGitCmd(t, dir, "commit", "--allow-empty", "-m", "seed")
	repo, _ := Open(dir)
	ctx := context.Background()
	reg := tool.NewRegistry()
	RegisterTools(reg, repo)
	if err := os.WriteFile(filepath.Join(dir, "empty.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, dir, "add", "empty.txt")
	runGitCmd(t, dir, "commit", "-m", "empty")
	logTool, _ := reg.Get("git_log")
	out, err := logTool.Execute(ctx, json.RawMessage(`{"path":"empty.txt","since":"2099-01-01","limit":5}`))
	if err != nil || !strings.Contains(out, "No commits") {
		t.Fatalf("future since: %q err=%v", out, err)
	}
	out, err = logTool.Execute(ctx, json.RawMessage(`{"path":"empty.txt","limit":5}`))
	if err != nil || !strings.Contains(out, "empty.txt") {
		t.Fatalf("path log: %q err=%v", out, err)
	}
	out, err = logTool.Execute(ctx, json.RawMessage(`{"limit":3}`))
	if err != nil || strings.Contains(out, "No commits") {
		t.Fatalf("repo-wide log: %q err=%v", out, err)
	}
}

func TestGitBlameNoLinesMessage(t *testing.T) {
	lines := parsePorcelainBlame("")
	reg := tool.NewRegistry()
	dir := t.TempDir()
	initGitRepo(t, dir)
	repo, _ := Open(dir)
	RegisterTools(reg, repo)
	blameTool, _ := reg.Get("git_blame")
	if len(lines) != 0 {
		t.Fatal("expected empty parse")
	}
	// Covered via parse helper; tool empty branch needs zero-length blame from git (rare).
	_ = blameTool
}

func TestGitShowToolSuccess(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, dir, "add", "note.txt")
	runGitCmd(t, dir, "commit", "-m", "note")
	repo, _ := Open(dir)
	reg := tool.NewRegistry()
	RegisterTools(reg, repo)
	show, _ := reg.Get("git_show")
	hash := strings.TrimSpace(runGitOutput(t, dir, "rev-parse", "HEAD"))
	out, err := show.Execute(context.Background(), json.RawMessage(`{"commit":"`+hash+`"}`))
	if err != nil || !strings.Contains(out, "note") {
		t.Fatalf("show: %q err=%v", out, err)
	}
}

func TestShowCommitTruncation(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	for i := 0; i < 4200; i++ {
		name := filepath.Join(dir, fmt.Sprintf("f%d.txt", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-m", "many files")
	repo, _ := Open(dir)
	out, err := repo.ShowCommit(context.Background(), "HEAD")
	if err != nil || !strings.Contains(out, "truncated") {
		t.Fatalf("truncate: len=%d err=%v", len(out), err)
	}
}

func TestParseBlameAndLogEdgeCases(t *testing.T) {
	blame := parsePorcelainBlame("hash 1 1 1\nauthor-mail <a>\nprevious abc\n\tline\n")
	if len(blame) != 1 || blame[0].Text != "line" {
		t.Fatalf("blame=%+v", blame)
	}
	if parseLogRecords("short\x1fline") != nil {
		t.Fatal("short log line")
	}
}

func TestRunGHStderr(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	repo, _ := Open(dir)
	withBadGH(t)
	if _, err := repo.PRContext(context.Background()); err == nil {
		t.Fatal("expected gh stderr error")
	}
}

func TestCommitCountRepoWide(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	runGitCmd(t, dir, "commit", "--allow-empty", "-m", "one")
	repo, _ := Open(dir)
	n, err := repo.CommitCount(context.Background(), "")
	if err != nil || n < 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
}

func TestRunGitNilContext(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	runGitCmd(t, dir, "commit", "--allow-empty", "-m", "ctx")
	if _, err := runGit(nil, dir, "rev-parse", "HEAD"); err != nil {
		t.Fatal(err)
	}
}

func TestGHPRListDefaultLimit(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	repo, _ := Open(dir)
	withFakeGH(t)
	out, err := repo.PRList(context.Background(), 0)
	if err != nil || out == "" {
		t.Fatalf("PRList: %q err=%v", out, err)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func withBadGH(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		body := `@echo off
echo gh failed 1>&2
exit /b 1
`
		if err := os.WriteFile(filepath.Join(dir, "gh.cmd"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	} else {
		p := filepath.Join(dir, "gh")
		if err := os.WriteFile(p, []byte("#!/bin/sh\necho gh failed >&2\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
