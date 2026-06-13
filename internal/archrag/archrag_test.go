package archrag

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"arcdesk/internal/tool"
)

func TestIndexAndTools(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "docs", "SPEC.md")
	if err := os.MkdirAll(filepath.Dir(spec), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# ArcDesk Spec\n\n## Security\n\nNever commit secrets.\n\n## API\n\nUse tools.\n"
	if err := os.WriteFile(spec, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Readme\n\nHello"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := NewIndex(dir, nil)
	if len(idx.List()) < 2 {
		t.Fatalf("summary = %v", idx.List())
	}
	sections := idx.FindSections("", "security", 5)
	if len(sections) == 0 {
		t.Fatal("expected security section")
	}
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	listTool, ok := reg.Get("architecture_list")
	if !ok {
		t.Fatal("missing architecture_list")
	}
	if _, err := listTool.Execute(context.Background(), json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	findTool, _ := reg.Get("architecture_find")
	out, err := findTool.Execute(context.Background(), json.RawMessage(`{"query":"API"}`))
	if err != nil || out == "" {
		t.Fatalf("find = %q err=%v", out, err)
	}
	readTool, _ := reg.Get("architecture_read")
	if _, err := readTool.Execute(context.Background(), json.RawMessage(`{"doc":"README.md"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestParseSections(t *testing.T) {
	text := "## One\n\nbody1\n### Two\n\nbody2\n"
	sections := parseSections("x.md", text)
	if len(sections) != 2 || sections[0].Heading != "One" || sections[1].Heading != "Two" {
		t.Fatalf("sections = %+v", sections)
	}
}
