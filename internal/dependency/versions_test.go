package dependency

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectVersionConflictsDuplicateRequire(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module root.test\n\ngo 1.21\n")
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "go.mod"), "module sub.test\n\ngo 1.21\n\nrequire github.com/foo/bar v1.0.0\n")
	writeFile(t, filepath.Join(root, "go.mod"), "module root.test\n\ngo 1.21\n\nrequire github.com/foo/bar v2.0.0\n")

	conflicts := detectVersionConflicts(root, nil)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %+v, want 1", conflicts)
	}
	if len(conflicts[0].Versions) < 2 {
		t.Fatalf("versions = %+v", conflicts[0].Versions)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestComputeFingerprintIncludesPackageJSON(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module x\n\ngo 1.21\n")
	writeFile(t, filepath.Join(root, "package.json"), `{"name":"x"}`)
	time.Sleep(20 * time.Millisecond)
	fp1 := ComputeFingerprint(root)
	if fp1 == "" {
		t.Fatal("expected fingerprint")
	}
	writeFile(t, filepath.Join(root, "package.json"), `{"name":"x","version":"2"}`)
	time.Sleep(20 * time.Millisecond)
	fp2 := ComputeFingerprint(root)
	if fp1 == fp2 {
		t.Fatal("fingerprint should change when package.json changes")
	}
}

func TestCheckStaleIndexVersion(t *testing.T) {
	meta := &Meta{IndexVersion: IndexVersion - 1, Fingerprint: "x"}
	if !CheckStale(t.TempDir(), meta) {
		t.Fatal("expected stale for old index version")
	}
}
