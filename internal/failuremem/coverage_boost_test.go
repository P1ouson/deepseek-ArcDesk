package failuremem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/tool"
)

func TestOpenAndRecordValidation(t *testing.T) {
	if _, err := Open("", 10); err == nil {
		t.Fatal("empty workspace")
	}
	root := t.TempDir()
	store, err := Open(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Record(Entry{Signature: "sig", Fix: "fix"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Record(Entry{Signature: "", Fix: "x"}); err == nil {
		t.Fatal("signature required")
	}
	if err := store.Record(Entry{Signature: "x", Fix: ""}); err == nil {
		t.Fatal("fix required")
	}
	var nilStore *Store
	if err := nilStore.Record(Entry{Signature: "a", Fix: "b"}); err == nil {
		t.Fatal("nil store")
	}
}

func TestMaxEntriesAndTruncation(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 2)
	long := strings.Repeat("e", 2500)
	if err := store.Record(Entry{Signature: "one", Fix: "f1", Error: long}); err != nil {
		t.Fatal(err)
	}
	if err := store.Record(Entry{Signature: "two", Fix: "f2"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Record(Entry{Signature: "three", Fix: "f3"}); err != nil {
		t.Fatal(err)
	}
	all, err := store.List(10)
	if err != nil || len(all) != 2 {
		t.Fatalf("truncate entries: %d err=%v", len(all), err)
	}
	if len(all[0].Error) > 2000 {
		t.Fatal("error not truncated")
	}
}

func TestSearchPathsAndTags(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 20)
	_ = store.Record(Entry{
		Signature: "vitest ui",
		Fix:       "mock fetch",
		Paths:     []string{"src/foo.test.ts"},
		Tags:      []string{"frontend"},
	})
	hits, err := store.Search("foo.test", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("path search: %v err=%v", hits, err)
	}
	hits, err = store.Search("frontend", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("tag search: %v err=%v", hits, err)
	}
	hits, err = store.Search("", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("empty query list: %v err=%v", hits, err)
	}
}

func TestLoadSkipsBadLines(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 10)
	dir, _ := store.path()
	if err := os.WriteFile(dir, []byte("not json\n{\"ts\":\"2020-01-01T00:00:00Z\",\"signature\":\"ok\",\"error\":\"\",\"fix\":\"done\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := store.List(10)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
}

func TestFailureMemToolsEdgeCases(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 10)
	reg := tool.NewRegistry()
	RegisterTools(reg, store)
	ctx := context.Background()
	list, _ := reg.Get("failuremem_list")
	out, err := list.Execute(ctx, json.RawMessage(`{"limit":5}`))
	if err != nil || !strings.Contains(out, "No failure memory") {
		t.Fatalf("empty list: %q err=%v", out, err)
	}
	search, _ := reg.Get("failuremem_search")
	out, err = search.Execute(ctx, json.RawMessage(`{"query":"missing","limit":3}`))
	if err != nil || !strings.Contains(out, "No failure memory matched") {
		t.Fatalf("no match: %q err=%v", out, err)
	}
	if _, err := search.Execute(ctx, json.RawMessage(`{`)); err == nil {
		t.Fatal("bad search json")
	}
	record, _ := reg.Get("failuremem_record")
	if _, err := record.Execute(ctx, json.RawMessage(`{"signature":"s","fix":"f","paths":["a"],"tags":["t"]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := record.Execute(ctx, json.RawMessage(`{`)); err == nil {
		t.Fatal("bad record json")
	}
}

func TestRegisterToolsNil(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterTools(nil, nil)
	RegisterTools(reg, nil)
	if reg.Len() != 0 {
		t.Fatal("no tools expected")
	}
}

func TestRecordPreservesTimestamp(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 10)
	ts := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := store.Record(Entry{TS: ts, Signature: "cmd", Fix: "patch"}); err != nil {
		t.Fatal(err)
	}
	entries, _ := store.List(1)
	if entries[0].TS != ts {
		t.Fatalf("ts=%v", entries[0].TS)
	}
}

func TestListNilStore(t *testing.T) {
	var s *Store
	if _, err := s.List(5); err == nil {
		t.Fatal("nil list")
	}
	if _, err := s.Search("q", 5); err == nil {
		t.Fatal("nil search")
	}
}

func TestFailureMemFileRoundTrip(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 10)
	if err := store.Record(Entry{Signature: "go test", Fix: "import"}); err != nil {
		t.Fatal(err)
	}
	p, err := store.path()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("missing file %s: %v", p, err)
	}
	if filepath.Base(p) != fileName {
		t.Fatalf("file name = %s", p)
	}
}

func TestFailureMemToolsMetadata(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 10)
	reg := tool.NewRegistry()
	RegisterTools(reg, store)
	for _, name := range []string{"failuremem_search", "failuremem_list", "failuremem_record"} {
		toolDef, ok := reg.Get(name)
		if !ok {
			t.Fatalf("missing %q", name)
		}
		if toolDef.Description() == "" {
			t.Fatalf("%q Description empty", name)
		}
		readOnly := name != "failuremem_record"
		if toolDef.ReadOnly() != readOnly {
			t.Fatalf("%q ReadOnly=%v want %v", name, toolDef.ReadOnly(), readOnly)
		}
	}
}

func TestSearchMatchBranches(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 20)
	_ = store.Record(Entry{Signature: "sig-a", Error: "needle-error", Fix: "f1"})
	_ = store.Record(Entry{Signature: "sig-b", Error: "e", Fix: "needle-fix"})
	_ = store.Record(Entry{Signature: "sig-c", Error: "e", Fix: "f", Paths: []string{"pkg/needle.go"}, Tags: []string{"ci"}})
	if hits, err := store.Search("needle-error", 5); err != nil || len(hits) != 1 {
		t.Fatalf("error match: %v err=%v", hits, err)
	}
	if hits, err := store.Search("needle-fix", 5); err != nil || len(hits) != 1 {
		t.Fatalf("fix match: %v err=%v", hits, err)
	}
	if hits, err := store.Search("needle.go", 5); err != nil || len(hits) != 1 {
		t.Fatalf("path match: %v err=%v", hits, err)
	}
	if hits, err := store.Search("ci", 5); err != nil || len(hits) != 1 {
		t.Fatalf("tag match: %v err=%v", hits, err)
	}
}

func TestRecordTruncatesFix(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 10)
	longFix := strings.Repeat("f", 2500)
	if err := store.Record(Entry{Signature: "cmd", Fix: longFix}); err != nil {
		t.Fatal(err)
	}
	entries, _ := store.List(1)
	if len(entries[0].Fix) > 2000 {
		t.Fatal("fix not truncated")
	}
}

func TestListLimitTrim(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 10)
	for i := 0; i < 5; i++ {
		if err := store.Record(Entry{Signature: "s" + string(rune('a'+i)), Fix: "f"}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := store.List(2)
	if err != nil || len(entries) != 2 {
		t.Fatalf("list trim: %d err=%v", len(entries), err)
	}
}

func TestOpenDefaultMaxEntries(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_ = store.Record(Entry{Signature: "s", Fix: "f"})
	}
	entries, _ := store.List(0)
	if len(entries) != 3 {
		t.Fatalf("default list limit: %d", len(entries))
	}
}

func TestLoadLockedInvalidFile(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 10)
	p, err := store.path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(5); err == nil {
		t.Fatal("expected load error for directory path")
	}
}

func TestSaveLockedFailure(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 10)
	if err := store.Record(Entry{Signature: "first", Fix: "ok"}); err != nil {
		t.Fatal(err)
	}
	p, err := store.path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := store.Record(Entry{Signature: "second", Fix: "fail"}); err == nil {
		t.Fatal("expected save failure on read-only file")
	}
}

func TestFailureMemListToolWithEntries(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 10)
	_ = store.Record(Entry{Signature: "go test", Fix: "patch"})
	reg := tool.NewRegistry()
	RegisterTools(reg, store)
	list, _ := reg.Get("failuremem_list")
	out, err := list.Execute(context.Background(), json.RawMessage(`{"limit":10}`))
	if err != nil || !strings.Contains(out, "recent entr") {
		t.Fatalf("list tool: %q err=%v", out, err)
	}
}

func TestOpenWorkspaceFileFails(t *testing.T) {
	f := filepath.Join(t.TempDir(), "notadir.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(f, 10); err == nil {
		t.Fatal("file path workspace should fail ProjectDir")
	}
}

func TestStorePathErrors(t *testing.T) {
	s := &Store{root: "", maxEntries: 10}
	if _, err := s.path(); err == nil {
		t.Fatal("empty root path")
	}
	if _, err := s.List(5); err == nil {
		t.Fatal("nil configured list")
	}
}

func TestSaveLockedRenameFailure(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 10)
	if err := store.Record(Entry{Signature: "first", Fix: "ok"}); err != nil {
		t.Fatal(err)
	}
	p, err := store.path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := store.Record(Entry{Signature: "second", Fix: "x"}); err == nil {
		t.Fatal("expected rename failure onto read-only target")
	}
}

func TestLoadScannerError(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root, 10)
	p, err := store.path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(strings.Repeat("x", 600*1024)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(5); err == nil {
		t.Fatal("expected scanner error for oversized line")
	}
}
