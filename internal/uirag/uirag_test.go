package uirag

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildIndexFindsExports(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "frontend", "src", "components")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "export function ChatPanel() { return null }\nexport const Sidebar = () => null\n"
	if err := os.WriteFile(filepath.Join(src, "Chat.tsx"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := BuildIndex(dir)
	if len(idx.Components) < 2 {
		t.Fatalf("components = %+v", idx.Components)
	}
	found := idx.Find("chat", 5)
	if len(found) == 0 {
		t.Fatalf("find chat: %+v", idx.Components)
	}
}

func TestDiscoverable(t *testing.T) {
	dir := t.TempDir()
	if Discoverable(dir) {
		t.Fatal("expected false")
	}
	src := filepath.Join(dir, "frontend", "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if Discoverable(dir) {
		t.Fatal("expected false without ui files")
	}
	if err := os.WriteFile(filepath.Join(src, "App.tsx"), []byte("export function App() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Discoverable(dir) {
		t.Fatal("expected true")
	}
}
