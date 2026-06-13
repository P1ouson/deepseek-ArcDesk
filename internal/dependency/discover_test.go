package dependency

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverableGoMod(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Discoverable(root) {
		t.Fatal("expected discoverable with go.mod")
	}
}

func TestDiscoverablePackageJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Discoverable(root) {
		t.Fatal("expected discoverable with package.json")
	}
}

func TestDiscoverableNestedPackageJSON(t *testing.T) {
	root := t.TempDir()
	front := filepath.Join(root, "frontend")
	if err := os.MkdirAll(front, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(front, "package.json"), []byte(`{"name":"front"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Discoverable(root) {
		t.Fatal("expected nested package.json to be discoverable")
	}
}

func TestDiscoverableEmpty(t *testing.T) {
	if Discoverable(t.TempDir()) {
		t.Fatal("empty dir should not be discoverable")
	}
}
