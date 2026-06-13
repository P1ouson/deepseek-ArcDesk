package uirag

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestLookupAndFind(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "frontend", "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(src, "Panel.tsx")
	body := "export function ChatPanel() {}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := BuildIndex(dir)
	if c, ok := idx.Lookup("ChatPanel"); !ok || c.Name != "ChatPanel" {
		t.Fatalf("lookup name: %+v ok=%v", c, ok)
	}
	if c, ok := idx.Lookup("frontend/src/Panel.tsx"); !ok {
		t.Fatalf("lookup path: %+v", c)
	}
	if _, ok := idx.Lookup("missing"); ok {
		t.Fatal("expected miss")
	}
	all := idx.Find("", 10)
	if len(all) != 1 {
		t.Fatalf("empty query list = %d", len(all))
	}
}

func TestBuildIndexSkipsNoise(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "frontend", "src")
	tests := filepath.Join(src, "__tests__")
	if err := os.MkdirAll(tests, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(tests, "App.test.tsx"), []byte("export function T() {}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "App.tsx"), []byte("export function App() {}\n"), 0o644)
	idx := BuildIndex(dir)
	if len(idx.Components) != 1 || idx.Components[0].Name != "App" {
		t.Fatalf("components = %+v", idx.Components)
	}
}

func TestScanRootsEmpty(t *testing.T) {
	if scanRoots("  ") != nil {
		t.Fatal("empty root")
	}
}

func TestUIRagToolsExecute(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "frontend", "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(src, "Chat.tsx")
	content := "export function ChatPanel() {\n  return null\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := BuildIndex(dir)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	for _, name := range []string{"ui_status", "ui_list", "ui_find", "ui_read"} {
		tl, ok := reg.Get(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if tl.Name() == "" || tl.Description() == "" || !tl.ReadOnly() {
			t.Fatalf("meta for %s", name)
		}
	}
	status, _ := reg.Get("ui_status")
	out, err := status.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "UI index") {
		t.Fatalf("status=%q err=%v", out, err)
	}
	list, _ := reg.Get("ui_list")
	out, err = list.Execute(context.Background(), json.RawMessage(`{"limit":5}`))
	if err != nil || !strings.Contains(out, "ChatPanel") {
		t.Fatalf("list=%q err=%v", out, err)
	}
	find, _ := reg.Get("ui_find")
	out, err = find.Execute(context.Background(), json.RawMessage(`{"query":"chat","limit":5}`))
	if err != nil || !strings.Contains(out, "ChatPanel") {
		t.Fatalf("find=%q err=%v", out, err)
	}
	out, err = find.Execute(context.Background(), json.RawMessage(`{"query":"nope"}`))
	if err != nil || !strings.Contains(out, "No UI components") {
		t.Fatalf("find empty=%q err=%v", out, err)
	}
	read, _ := reg.Get("ui_read")
	out, err = read.Execute(context.Background(), json.RawMessage(`{"name_or_path":"ChatPanel","offset":0,"limit":2}`))
	if err != nil || !strings.Contains(out, "ChatPanel") || !strings.Contains(out, "1|") {
		t.Fatalf("read=%q err=%v", out, err)
	}
	out, err = read.Execute(context.Background(), json.RawMessage(`{"name_or_path":"ChatPanel","offset":99}`))
	if err != nil || !strings.Contains(out, "beyond file") {
		t.Fatalf("read offset=%q err=%v", out, err)
	}
	if _, err = read.Execute(context.Background(), json.RawMessage(`{"name_or_path":"missing"}`)); err == nil {
		t.Fatal("expected not found error")
	}
	if _, err = read.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected invalid args")
	}
}

func TestUIRagListEmptyIndex(t *testing.T) {
	idx := BuildIndex(t.TempDir())
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	list, _ := reg.Get("ui_list")
	out, err := list.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "No UI components") {
		t.Fatalf("list empty=%q err=%v", out, err)
	}
}

func TestUIRagRegisterNilSafe(t *testing.T) {
	RegisterTools(nil, BuildIndex(t.TempDir()))
	RegisterTools(tool.NewRegistry(), nil)
}

func TestIndexNilSafe(t *testing.T) {
	var idx *Index
	if idx.Find("x", 1) != nil {
		t.Fatal("nil find")
	}
	if _, ok := idx.Lookup("x"); ok {
		t.Fatal("nil lookup")
	}
}
