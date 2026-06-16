package toolcache

import (
	"testing"

	"arcdesk/internal/toolstats"
)

func TestInvalidatePathsKeepsUnrelatedReads(t *testing.T) {
	c := New()
	c.Put("read_file", `{"path":"a.go"}`, Entry{Output: "a"})
	c.Put("read_file", `{"path":"b.go"}`, Entry{Output: "b"})

	removed := c.InvalidatePaths([]string{"a.go"})
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if got, ok := c.Get("read_file", `{"path":"a.go"}`); ok || got.Output != "" {
		t.Fatal("expected a.go cache miss")
	}
	if got, ok := c.Get("read_file", `{"path":"b.go"}`); !ok || got.Output != "b" {
		t.Fatalf("expected b.go hit, got ok=%v out=%q", ok, got.Output)
	}
}

func TestInvalidatePathsEmptyClearsAll(t *testing.T) {
	c := New()
	c.Put("read_file", `{"path":"a.go"}`, Entry{Output: "a"})
	if removed := c.InvalidatePaths(nil); removed != 1 {
		t.Fatalf("removed = %d", removed)
	}
	if _, ok := c.Get("read_file", `{"path":"a.go"}`); ok {
		t.Fatal("expected full invalidate")
	}
}

func TestCachePathsReadFile(t *testing.T) {
	paths := CachePaths("read_file", `{"path":"src/main.go"}`, toolstats.KeyContext{WorkDir: "/ws"})
	if len(paths) != 1 || paths[0] != "src/main.go" {
		t.Fatalf("paths = %#v", paths)
	}
}
