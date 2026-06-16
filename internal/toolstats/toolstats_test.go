package toolstats

import "testing"

func TestCanonicalArgsIgnoresKeyOrder(t *testing.T) {
	a := CanonicalArgs(`{"path":"x","offset":1}`)
	b := CanonicalArgs(`{"offset":1,"path":"x"}`)
	if a != b {
		t.Fatalf("canonical mismatch:\n%q\n%q", a, b)
	}
}

func TestTrackerDuplicates(t *testing.T) {
	tr := NewTracker()
	tr.ResetTurn()
	tr.Record("read_file", `{"path":"a.go"}`, true)
	tr.Record("read_file", `{"path":"a.go"}`, true)
	tr.Record("bash", `{"command":"go test ./..."}`, false)

	turn := tr.Turn()
	if turn.Calls != 3 || turn.Duplicates != 1 {
		t.Fatalf("turn = %+v, want 3 calls 1 duplicate", turn)
	}
	if turn.CacheableCalls != 2 || turn.CacheableDupes != 1 {
		t.Fatalf("turn cacheable = %+v, want 2 calls 1 dupe", turn)
	}

	tr.ResetTurn()
	if tr.Turn().Calls != 0 {
		t.Fatal("ResetTurn should clear turn stats")
	}
	sess := tr.Session()
	if sess.Calls != 3 || sess.Duplicates != 1 {
		t.Fatalf("session = %+v, want preserved totals", sess)
	}
}

func TestDuplicateRate(t *testing.T) {
	s := Stats{Calls: 4, Duplicates: 1}
	if got := s.DuplicateRate(); got != 0.25 {
		t.Fatalf("DuplicateRate() = %v, want 0.25", got)
	}
}
