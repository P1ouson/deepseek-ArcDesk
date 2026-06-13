package runtime

import "testing"

func TestNormalizeKindWails(t *testing.T) {
	if got := normalizeKind(KindGoLog, "wails-dev", "bind failed", nil); got != KindWails {
		t.Fatalf("got %q", got)
	}
	if got := normalizeKind(KindConsole, "console", "err", nil); got != KindConsole {
		t.Fatalf("got %q", got)
	}
}
