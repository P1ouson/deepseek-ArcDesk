package repomap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectDirRequiresWorkspace(t *testing.T) {
	if _, err := ProjectDir(""); err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestComposeUsesBlockWhenBaseEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	got := Compose("", root)
	if !strings.Contains(got, "Project repository map") {
		t.Fatalf("got %q", got)
	}
}

func TestLoadBlockEmptyWorkspace(t *testing.T) {
	if got := LoadBlock("  "); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadBlockCorruptReadIndex(t *testing.T) {
	root := t.TempDir()
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	p, err := readIndexPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	block := LoadBlock(root)
	if !strings.Contains(block, "Top-level layout") {
		t.Fatalf("expected repo map despite corrupt read index: %q", block)
	}
}

func TestRecordReadTruncatesSummaryAndUpdates(t *testing.T) {
	root := t.TempDir()
	long := strings.Repeat("x", 200)
	if err := RecordRead(root, "a.go", long); err != nil {
		t.Fatal(err)
	}
	rows := loadReadIndex(root)
	if len(rows) != 1 || len(rows[0].Summary) != 160 {
		t.Fatalf("rows=%+v", rows)
	}
	if err := RecordRead(root, "a.go", "updated"); err != nil {
		t.Fatal(err)
	}
	rows = loadReadIndex(root)
	if len(rows) != 1 || rows[0].Summary != "updated" {
		t.Fatalf("rows=%+v", rows)
	}
	if err := RecordRead("", "a.go", "x"); err != nil {
		t.Fatal(err)
	}
}

func TestRecordReadWithoutSummaryInBlock(t *testing.T) {
	root := t.TempDir()
	if err := RecordRead(root, "plain.go", ""); err != nil {
		t.Fatal(err)
	}
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	block := LoadBlock(root)
	if !strings.Contains(block, "`plain.go`") || strings.Contains(block, "`plain.go` —") {
		t.Fatalf("block=%q", block)
	}
}

func TestRecordExploreSummarySkipsEmpty(t *testing.T) {
	if err := RecordExploreSummary(t.TempDir(), "", "summary"); err != nil {
		t.Fatal(err)
	}
	if err := RecordExploreSummary(t.TempDir(), "task", ""); err != nil {
		t.Fatal(err)
	}
}

func TestRecordExploreSummaryTruncates(t *testing.T) {
	root := t.TempDir()
	task := strings.Repeat("t", 200)
	summary := strings.Repeat("s", 500)
	if err := RecordExploreSummary(root, task, summary); err != nil {
		t.Fatal(err)
	}
	body := loadExploreSummariesBody(root)
	if len(body) > 600 {
		t.Fatalf("body too long: %d", len(body))
	}
}

func TestIsStaleUsesFingerprint(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	meta, err := loadMeta(root)
	if err != nil || meta.Fingerprint == "" {
		t.Fatalf("meta=%+v err=%v", meta, err)
	}
}

func TestIsStaleUsesGitHead(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "t@example.com")
	runGit(t, root, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# git"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "init")
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	stale, err := isStale(root)
	if err != nil || stale {
		t.Fatalf("stale=%v err=%v", stale, err)
	}
}

func TestLoadMetaCorruptJSON(t *testing.T) {
	root := t.TempDir()
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	mp, err := metaPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mp, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadMeta(root); err == nil {
		t.Fatal("expected corrupt meta error")
	}
	stale, err := isStale(root)
	if !stale {
		t.Fatalf("stale=%v err=%v", stale, err)
	}
}

func TestRefreshIfStaleRebuildsAfterChange(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "t@example.com")
	runGit(t, root, "config", "user.name", "test")
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("# v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "v1")
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readme, []byte("# v2 changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "v2")
	if err := RefreshIfStale(root); err != nil {
		t.Fatal(err)
	}
	block := LoadBlock(root)
	if !strings.Contains(block, "v2 changed") {
		t.Fatalf("block=%q", block)
	}
}

func TestRecordReadCapsAtMaxRows(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < maxReadIndexRows+5; i++ {
		if err := RecordRead(root, fmt.Sprintf("file%d.go", i), "s"); err != nil {
			t.Fatal(err)
		}
	}
	rows := loadReadIndex(root)
	if len(rows) != maxReadIndexRows {
		t.Fatalf("rows=%d want %d", len(rows), maxReadIndexRows)
	}
}

func TestReadSnippetTruncates(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "big.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 3000)), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readSnippet(path, 100)
	if !strings.Contains(got, "…") || len(got) > 110 {
		t.Fatalf("got len=%d", len(got))
	}
}

func TestIsStaleDetectsGitHeadChange(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "t@example.com")
	runGit(t, root, "config", "user.name", "test")
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("# one"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "one")
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readme, []byte("# two"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "two")
	stale, err := isStale(root)
	if err != nil || !stale {
		t.Fatalf("stale=%v err=%v", stale, err)
	}
}

func TestEnsureReadySkipsExistingMap(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureReady(root); err != nil {
		t.Fatal(err)
	}
	if err := EnsureReady(root); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshIfStaleEmptyWorkspace(t *testing.T) {
	if err := RefreshIfStale(""); err != nil {
		t.Fatal(err)
	}
}

func TestRepoRevisionFingerprint(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	head, fp := repoRevision(root)
	if head != "" || fp == "" {
		t.Fatalf("head=%q fp=%q", head, fp)
	}
}

func TestSkipDirAndListTree(t *testing.T) {
	if !skipDir(".git") || !skipDir("node_modules") || skipDir("src") {
		t.Fatal("skipDir mismatch")
	}
	root := t.TempDir()
	for _, name := range []string{"src", ".git", "README.md"} {
		path := filepath.Join(root, name)
		if name == "src" || name == ".git" {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lines, err := listTree(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, ".git") {
		t.Fatalf("listed skipped dir: %q", joined)
	}
	if !strings.Contains(joined, "README.md") || !strings.Contains(joined, "src/") {
		t.Fatalf("lines=%q", joined)
	}
}

func TestReadSnippetMissingFile(t *testing.T) {
	if got := readSnippet(filepath.Join(t.TempDir(), "missing"), 100); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestGenerateMapIncludesManifests(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("all:\n\ttrue"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := generateMap(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "go.mod") || !strings.Contains(body, "Makefile") {
		t.Fatalf("body=%q", body)
	}
}

func TestPathHelpersRejectEmptyWorkspace(t *testing.T) {
	for _, fn := range []func(string) (string, error){
		mapPath, metaPath, readIndexPath,
	} {
		if _, err := fn(""); err == nil {
			t.Fatal("expected error for empty workspace")
		}
	}
}

func TestEnsureReadyReturnsProjectDirError(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureReady(f); err == nil {
		t.Fatal("expected EnsureReady error for file workspace")
	}
}

func TestRefreshIfStaleReturnsMetaError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := refresh(root); err != nil {
		t.Fatal(err)
	}
	mp, err := metaPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mp, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RefreshIfStale(root); err == nil {
		t.Fatal("expected meta parse error")
	}
}

func TestRefreshReturnsProjectDirError(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := refresh(f); err == nil {
		t.Fatal("expected refresh error")
	}
}

func TestRepoRevisionNonDirectory(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	head, fp := repoRevision(f)
	if head != "" || fp != "" {
		t.Fatalf("head=%q fp=%q", head, fp)
	}
}

func TestListTreeRespectsMaxDepth(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c.txt")
	if err := os.MkdirAll(filepath.Dir(deep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := listTree(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "c.txt") {
		t.Fatalf("deep file should be excluded: %q", joined)
	}
}

func TestIsStaleWithoutMapFile(t *testing.T) {
	root := t.TempDir()
	stale, err := isStale(root)
	if err != nil || !stale {
		t.Fatalf("stale=%v err=%v", stale, err)
	}
}

func TestLoadExploreSummariesBlankFile(t *testing.T) {
	root := t.TempDir()
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, exploreSummariesName)
	if err := os.WriteFile(path, []byte("  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadExploreSummariesBody(root); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestEnsureReadyEmptyWorkspace(t *testing.T) {
	if err := EnsureReady(""); err != nil {
		t.Fatal(err)
	}
}

func TestMapPathReturnsRepoMapFile(t *testing.T) {
	p, err := mapPath(t.TempDir())
	if err != nil || !strings.HasSuffix(p, mapFileName) {
		t.Fatalf("path=%q err=%v", p, err)
	}
}

func TestEnsureReadyConcurrent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() { done <- EnsureReady(root) }()
	}
	for i := 0; i < 4; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestListTreeTruncatesManyEntries(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 125; i++ {
		name := fmt.Sprintf("file_%03d.txt", i)
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lines, err := listTree(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) < 120 || !strings.Contains(lines[len(lines)-1], "truncated") {
		t.Fatalf("lines=%d last=%q", len(lines), lines[len(lines)-1])
	}
}

func TestGenerateMapWithPackageJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := generateMap(root)
	if err != nil || !strings.Contains(body, "package.json") {
		t.Fatalf("body=%q err=%v", body, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
