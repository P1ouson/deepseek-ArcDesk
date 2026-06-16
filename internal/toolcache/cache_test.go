package toolcache

import "testing"

func TestCacheHitMissAndInvalidate(t *testing.T) {
	c := New()
	e := Entry{Output: "hello"}
	c.Put("read_file", `{"path":"a.go"}`, e)

	if got, ok := c.Get("read_file", `{"path":"a.go"}`); !ok || got.Output != "hello" {
		t.Fatalf("Get = %+v ok=%v", got, ok)
	}
	if _, ok := c.Get("read_file", `{"path":"b.go"}`); ok {
		t.Fatal("unexpected hit for different args")
	}
	c.RecordMiss()

	s := c.Snapshot()
	if s.SessionHits != 1 || s.SessionMisses != 1 {
		t.Fatalf("stats = %+v", s)
	}

	c.InvalidateAll()
	if _, ok := c.Get("read_file", `{"path":"a.go"}`); ok {
		t.Fatal("expected miss after invalidate")
	}
	if snap := c.Snapshot(); snap.SessionHits != 1 {
		t.Fatalf("hits after invalidate lookup = %+v", snap)
	}
}

func TestCacheableExcludesStatefulTools(t *testing.T) {
	if Cacheable("read_file", true) != true {
		t.Fatal("read_file should be cacheable")
	}
	if Cacheable("todo_write", true) {
		t.Fatal("todo_write must not be cacheable")
	}
	if Cacheable("bash", false) {
		t.Fatal("bash must not be cacheable")
	}
}

func TestResetTurnClearsTurnCountersOnly(t *testing.T) {
	c := New()
	c.Put("grep", `{"pattern":"x"}`, Entry{Output: "y"})
	c.Get("grep", `{"pattern":"x"}`)
	c.ResetTurn()
	if s := c.Snapshot(); s.TurnHits != 0 || s.SessionHits != 1 {
		t.Fatalf("ResetTurn = %+v", s)
	}
}
