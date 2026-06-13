package archrag

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestIndexEdgeCases(t *testing.T) {
	idx := NewIndex(t.TempDir(), []string{"", "docs/extra.md"})
	if len(idx.List()) != 0 {
		t.Fatalf("empty index docs=%v", idx.List())
	}
	if len(idx.FindSections("", "", 5)) != 0 {
		t.Fatal("empty index find")
	}
	if _, ok := idx.ReadDoc("nope.md"); ok {
		t.Fatal("nil index read")
	}
}

func TestDocWithoutHeadings(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "README.md")
	if err := os.WriteFile(plain, []byte("plain text no headings"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := NewIndex(dir, nil)
	if len(idx.List()) != 1 || idx.List()[0].Title == "" {
		t.Fatalf("summary=%v", idx.List())
	}
	sections := idx.FindSections("", "plain", 5)
	if len(sections) == 0 {
		t.Fatal("body search")
	}
}

func TestTruncateAndHeadingLevel(t *testing.T) {
	if truncate("short", 10) != "short" {
		t.Fatal("short truncate")
	}
	long := truncate(strings.Repeat("a", 20), 10)
	if !strings.Contains(long, "truncated") {
		t.Fatal("long truncate")
	}
	if headingLevel("not heading") != 0 {
		t.Fatal("not heading")
	}
	if headingLevel("####### bad") != 0 {
		t.Fatal("too many #")
	}
}

func TestArchToolsEdgeCases(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Title\n\nBody"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := NewIndex(dir, []string{"README.md"})
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	ctx := context.Background()
	find, _ := reg.Get("architecture_find")
	out, err := find.Execute(ctx, json.RawMessage(`{"query":"missing"}`))
	if err != nil || !strings.Contains(out, "No sections matched") {
		t.Fatalf("no match: %q err=%v", out, err)
	}
	if _, err := find.Execute(ctx, json.RawMessage(`{`)); err == nil {
		t.Fatal("bad find json")
	}
	read, _ := reg.Get("architecture_read")
	if _, err := read.Execute(ctx, json.RawMessage(`{"doc":"ghost.md"}`)); err == nil {
		t.Fatal("unindexed read")
	}
	if _, err := read.Execute(ctx, json.RawMessage(`{`)); err == nil {
		t.Fatal("bad read json")
	}
	findScoped, err := find.Execute(ctx, json.RawMessage(`{"query":"Title","doc":"README.md","limit":1}`))
	if err != nil || !strings.Contains(findScoped, "Title") {
		t.Fatalf("scoped find: %q err=%v", findScoped, err)
	}
}

func TestRegisterToolsNilAndEmpty(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterTools(nil, nil)
	RegisterTools(reg, nil)
	RegisterTools(reg, NewIndex(t.TempDir(), nil))
	if reg.Len() != 0 {
		t.Fatal("no tools for empty index")
	}
}

func TestReadDocOutsideIndex(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "other.md")
	if err := os.WriteFile(other, []byte("# Other"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Main"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := NewIndex(dir, nil)
	if _, ok := idx.ReadDoc("other.md"); ok {
		t.Fatal("file on disk but not in index summary")
	}
	text, ok := idx.ReadDoc("README.md")
	if !ok || !strings.Contains(text, "Main") {
		t.Fatalf("read=%q ok=%v", text, ok)
	}
}

func TestListToolEmptyIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	// empty file still indexes with synthetic section
	idx := NewIndex(dir, nil)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	list, _ := reg.Get("architecture_list")
	out, err := list.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil || !strings.Contains(out, "architecture document") {
		t.Fatalf("list: %q err=%v", out, err)
	}
}
