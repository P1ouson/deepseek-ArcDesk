package builtin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdaptiveReadLimitSmallRepo(t *testing.T) {
	dir := t.TempDir()
	writeLines(t, filepath.Join(dir, "main.go"), 350)

	setRepoTierOverride(func(string) (repoTier, bool) { return repoTierSmall, true })
	t.Cleanup(func() { setRepoTierOverride(nil) })

	if got := effectiveReadLimit(dir, filepath.Join(dir, "main.go"), 0, 0); got != readFileLimitSmall {
		t.Fatalf("small repo limit = %d, want %d", got, readFileLimitSmall)
	}
}

func TestAdaptiveReadLimitLargeEntryFile(t *testing.T) {
	dir := t.TempDir()
	writeLines(t, filepath.Join(dir, "README.md"), 350)

	setRepoTierOverride(func(string) (repoTier, bool) { return repoTierLarge, true })
	t.Cleanup(func() { setRepoTierOverride(nil) })

	if got := effectiveReadLimit(dir, filepath.Join(dir, "README.md"), 0, 0); got != readFileLimitMedium {
		t.Fatalf("large structure limit = %d, want %d", got, readFileLimitMedium)
	}
}

func TestAdaptiveReadLimitLargeAggressiveEntryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	writeLines(t, path, 350)

	setRepoTierOverride(func(string) (repoTier, bool) { return repoTierLarge, true })
	t.Cleanup(func() { setRepoTierOverride(nil) })

	if got := effectiveReadLimit(dir, path, 0, 0); got != readFileLimitLargeEntry {
		t.Fatalf("large aggressive entry limit = %d, want %d", got, readFileLimitLargeEntry)
	}
}

func TestWholeFileReadSmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "calc.py")
	os.WriteFile(path, []byte("def add(a,b):\n    return a+b\n"), 0o644)

	setRepoTierOverride(func(string) (repoTier, bool) { return repoTierLarge, true })
	t.Cleanup(func() { setRepoTierOverride(nil) })

	got := effectiveReadLimit(dir, path, 0, 0)
	if got != 2 {
		t.Fatalf("whole-file limit = %d, want 2 lines", got)
	}
}

func TestAdaptiveReadLimitLargeImplementationFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.go")
	var b strings.Builder
	for i := 0; i < 2000; i++ {
		b.WriteString("// implementation line\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	setRepoTierOverride(func(string) (repoTier, bool) { return repoTierLarge, true })
	t.Cleanup(func() { setRepoTierOverride(nil) })

	if got := effectiveReadLimit(dir, path, 0, 0); got != readFileLimitLargeImpl {
		t.Fatalf("large impl limit = %d, want %d", got, readFileLimitLargeImpl)
	}
}

func TestSmartExpansionAfterTwoPages(t *testing.T) {
	dir := t.TempDir()
	// Use a large implementation file path so whole-file shortcut does not apply.
	path := filepath.Join(dir, "service.go")
	writeLines(t, path, 400)

	setRepoTierOverride(func(string) (repoTier, bool) { return repoTierLarge, true })
	t.Cleanup(func() { setRepoTierOverride(nil) })

	if got := effectiveReadLimit(dir, path, 500, 0); got != readFileSmartExpandLarge {
		t.Fatalf("expanded limit = %d, want %d", got, readFileSmartExpandLarge)
	}
	if got := effectiveReadLimit(dir, path, 250, 0); got != readFileLimitSmall {
		t.Fatalf("second page should stay compact = %d, want %d", got, readFileLimitSmall)
	}
}

func TestAdaptiveReadLimitMediumEntryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	writeLines(t, path, 350)

	setRepoTierOverride(func(string) (repoTier, bool) { return repoTierMedium, true })
	t.Cleanup(func() { setRepoTierOverride(nil) })

	if got := effectiveReadLimit(dir, path, 0, 0); got != readFileLimitMedium {
		t.Fatalf("medium entry limit = %d, want %d", got, readFileLimitMedium)
	}
}

func TestExplicitLimitOverridesAdaptive(t *testing.T) {
	dir := t.TempDir()
	setRepoTierOverride(func(string) (repoTier, bool) { return repoTierLarge, true })
	t.Cleanup(func() { setRepoTierOverride(nil) })

	if got := effectiveReadLimit(dir, filepath.Join(dir, "README.md"), 0, 123); got != 123 {
		t.Fatalf("explicit limit = %d, want 123", got)
	}
}

func writeLines(t *testing.T, path string, n int) {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
