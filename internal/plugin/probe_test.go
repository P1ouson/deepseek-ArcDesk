package plugin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestProbeExecutableFindsNodeWhenOnPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("node may not be on PATH in CI Windows shells")
	}
	path, ok := ProbeExecutable("node")
	if !ok {
		t.Skip("node not installed in test environment")
	}
	if filepath.Base(path) == "" {
		t.Fatalf("empty path for node")
	}
	_, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat node: %v", err)
	}
}
