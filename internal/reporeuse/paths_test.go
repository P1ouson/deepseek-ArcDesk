package reporeuse

import "testing"

func TestPathsAffectDependency(t *testing.T) {
	if !PathsAffectDependency([]string{"internal/foo/bar.go"}) {
		t.Fatal("expected .go to affect dependency")
	}
	if PathsAffectDependency([]string{"docs/readme.md", "notes/todo.txt"}) {
		t.Fatal("docs-only change should not affect dependency")
	}
}

func TestPathsAffectCallgraph(t *testing.T) {
	if !PathsAffectCallgraph([]string{"desktop/main.go"}) {
		t.Fatal("desktop go file should affect callgraph")
	}
	if PathsAffectCallgraph([]string{"README.md"}) {
		t.Fatal("readme should not affect callgraph")
	}
}

func TestHeadChangedFingerprintStable(t *testing.T) {
	if !HeadChangedFingerprintStable("aaa", "bbb", "fp1", "fp1") {
		t.Fatal("same fingerprint with new head should qualify for meta bump")
	}
	if HeadChangedFingerprintStable("aaa", "aaa", "fp1", "fp1") {
		t.Fatal("unchanged head should not qualify")
	}
}
