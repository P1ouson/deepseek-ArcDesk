package toolstats

import (
	"path/filepath"
	"testing"
)

func TestNormalizeReadFilePathAliases(t *testing.T) {
	dir := t.TempDir()
	a := NormalizeToolArgs("read_file", `{"path":"./main.go"}`, dir)
	b := NormalizeToolArgs("read_file", `{"path":"main.go"}`, dir)
	if a != b {
		t.Fatalf("path aliases should normalize to same key:\n%q\n%q", a, b)
	}
}

func TestNormalizeReadFileStripsDefaultPaging(t *testing.T) {
	dir := t.TempDir()
	a := NormalizeToolArgs("read_file", `{"path":"main.go"}`, dir)
	b := NormalizeToolArgs("read_file", `{"path":"main.go","offset":0,"limit":0}`, dir)
	if a != b {
		t.Fatalf("default paging fields should be omitted:\n%q\n%q", a, b)
	}
}

func TestNormalizeReadFileKeepsDistinctPaging(t *testing.T) {
	dir := t.TempDir()
	a := NormalizeToolArgs("read_file", `{"path":"main.go","offset":0,"limit":50}`, dir)
	b := NormalizeToolArgs("read_file", `{"path":"main.go","offset":50,"limit":50}`, dir)
	if a == b {
		t.Fatal("different paging windows must not collapse")
	}
}

func TestIntentKeyMatchesEquivalentGrepPaths(t *testing.T) {
	dir := t.TempDir()
	ctx := KeyContext{WorkDir: dir, Normalize: true}
	a := IntentKey("grep", `{"pattern":"func","path":"."}`, ctx)
	b := IntentKey("grep", `{"pattern":"func"}`, ctx)
	if a != b {
		t.Fatalf("grep path defaults should match:\n%q\n%q", a, b)
	}
}

func TestIntentKeyDisabledUsesRawCanonical(t *testing.T) {
	dir := t.TempDir()
	ctx := KeyContext{WorkDir: dir, Normalize: false}
	a := IntentKey("read_file", `{"path":"./main.go"}`, ctx)
	b := IntentKey("read_file", `{"path":"main.go"}`, ctx)
	if a == b {
		t.Fatal("normalization disabled should keep distinct raw args")
	}
}

func TestTrackerNormalizedDupes(t *testing.T) {
	dir := t.TempDir()
	tr := NewTracker()
	tr.SetKeyContext(KeyContext{WorkDir: dir, Normalize: true})
	tr.Record("read_file", `{"path":"main.go"}`, true)
	tr.Record("read_file", `{"path":"./main.go"}`, true)
	sess := tr.Session()
	if sess.Duplicates != 1 {
		t.Fatalf("duplicates = %d, want 1", sess.Duplicates)
	}
	if sess.NormalizedDupes != 1 {
		t.Fatalf("normalized dupes = %d, want 1", sess.NormalizedDupes)
	}
}

func TestNormalizeGlobPatternSlashes(t *testing.T) {
	a := NormalizeToolArgs("glob", `{"pattern":"internal\\*.go"}`, "")
	b := NormalizeToolArgs("glob", `{"pattern":"internal/*.go"}`, "")
	if a != b {
		t.Fatalf("glob patterns should normalize slashes:\n%q\n%q", a, b)
	}
}

func TestNormalizePathKeyUsesAbsoluteForm(t *testing.T) {
	dir := filepath.Clean(t.TempDir())
	got := normalizePathKey(dir, "sub/file.go")
	want := normalizePathKey(dir, filepath.Join(dir, "sub", "file.go"))
	if got != want {
		t.Fatalf("relative and absolute paths should match:\n%q\n%q", got, want)
	}
}
