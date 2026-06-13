package checkpoint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"arcdesk/internal/diff"
	fileenc "arcdesk/internal/fileutil/encoding"
)

func TestSnapshotNoOps(t *testing.T) {
	root := t.TempDir()
	s := New("", root)

	s.Snapshot(diff.Change{Path: "", Kind: diff.Modify, OldText: "x"})
	s.Snapshot(diff.Change{Path: filepath.Join(root, "a.txt"), Kind: diff.Modify, OldText: "x"})

	s.Begin(0, "p", 0)
	s.Snapshot(diff.Change{Path: filepath.Join(root, "new.txt"), Kind: diff.Create})
	plan := s.RestorePlan(0)
	if len(plan.Targets) != 1 || plan.Targets[0].Content != nil {
		t.Fatalf("create snapshot = %+v", plan.Targets)
	}
}

func TestBoundsCurrentOnly(t *testing.T) {
	s := New("", t.TempDir())
	s.Begin(2, "only", 7)
	b := s.Bounds()
	if b[2] != 7 || len(b) != 1 {
		t.Fatalf("bounds = %v", b)
	}
}

func TestNextTurnVariants(t *testing.T) {
	s := New("", t.TempDir())
	if got := s.NextTurn(); got != 0 {
		t.Fatalf("empty NextTurn = %d", got)
	}
	s.Begin(3, "p", 0)
	if got := s.NextTurn(); got != 4 {
		t.Fatalf("cur NextTurn = %d", got)
	}
}

func TestLoadSkipsBadEntries(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	valid := Checkpoint{Turn: 0, Time: time.Now(), Prompt: "ok", MsgIndex: 0}
	b, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "turn-0.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "turn-1.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := New(dir, root)
	metas := s.List()
	if len(metas) != 1 || metas[0].Prompt != "ok" {
		t.Fatalf("metas = %+v", metas)
	}
}

func TestLoadReadDirFailure(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(blocker, root)
	if len(s.List()) != 0 {
		t.Fatal("expected empty store when dir is unreadable")
	}
}

func TestPersistDirCreationFailure(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(blocker, "ckpt")
	s := &Store{dir: dir, root: root, seen: map[string]bool{}}
	s.Begin(0, "persist fail", 0)
}

func TestPersistWriteFailure(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(t.TempDir(), "asfile")
	if err := os.WriteFile(dir, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Store{dir: dir, root: root, seen: map[string]bool{}}
	s.Begin(0, "write fail", 0)
}

func TestRestorePlanFutureTurn(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	write(t, a, "v0")
	s := New("", root)
	s.Begin(0, "first", 0)
	s.Snapshot(diff.Change{Path: a, Kind: diff.Modify, OldText: "v0"})

	plan := s.RestorePlan(99)
	if plan.FromTurn != 99 || len(plan.Targets) != 0 || plan.Prompt != "" {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestRestoreCodeDeleteMissingFile(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "gone.txt")
	s := New("", root)
	s.Begin(0, "p", 0)
	s.Snapshot(diff.Change{Path: missing, Kind: diff.Create})

	_, deleted, err := s.RestoreCode(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 0 {
		t.Fatalf("deleted = %v", deleted)
	}
}

func TestRestoreCodeUsesSnapshotEncoding(t *testing.T) {
	root := t.TempDir()
	dir := t.TempDir()
	a := filepath.Join(root, "enc.txt")
	original := "\u4f60\u597d"
	enc := fileenc.GB18030
	legacy := Checkpoint{
		Turn: 0, Time: time.Now(), Prompt: "enc", MsgIndex: 0,
		Files: []FileSnap{{
			Path: a, Content: &original, Encoding: &enc,
		}},
	}
	b, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "turn-0.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := New(dir, root)
	if _, _, err := s.RestoreCode(0); err != nil {
		t.Fatal(err)
	}
	got := readBytes(t, a)
	want := fileenc.Encode(original, fileenc.GB18030)
	if string(got) == original {
		t.Fatal("expected encoded bytes, got UTF-8 text")
	}
	if string(got) != string(want) {
		t.Fatalf("restored = %q, want encoded GB18030", got)
	}
}

func TestRestoreCodeMkdirFailure(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	write(t, blocker, "block")
	nested := filepath.Join("blocker", "nested", "f.txt")
	absNested := filepath.Join(root, nested)
	content := "snap"
	s := New("", root)
	s.Begin(0, "p", 0)
	s.Snapshot(diff.Change{Path: absNested, Kind: diff.Modify, OldText: content})

	_, _, err := s.RestoreCode(0)
	if err == nil {
		t.Fatal("expected mkdir failure")
	}
}

func TestSafePathAbsoluteAndEmptyRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "a.txt")
	got, err := safePath(root, inside)
	if err != nil || got != filepath.Clean(inside) {
		t.Fatalf("abs inside root: got=%q err=%v", got, err)
	}
	if _, err := safePath("", "rel/path"); err != nil {
		t.Fatalf("empty root: %v", err)
	}
}

func TestDetectCurrentEncodingMissingFile(t *testing.T) {
	if got := detectCurrentEncoding(filepath.Join(t.TempDir(), "missing.txt")); got != nil {
		t.Fatalf("got %v", got)
	}
}
