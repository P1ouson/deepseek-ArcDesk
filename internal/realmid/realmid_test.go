package realmid

import "testing"

func TestParseSimple(t *testing.T) {
	r, err := Parse("go:arcdesk/internal/dependency")
	if err != nil {
		t.Fatal(err)
	}
	if r.Realm != "go" || r.Path != "arcdesk/internal/dependency" || r.Symbol != "" || r.Line != 0 {
		t.Fatalf("got %+v", r)
	}
	if r.String() != "go:arcdesk/internal/dependency" {
		t.Fatalf("String() = %q", r.String())
	}
}

func TestParseStdlib(t *testing.T) {
	r, err := Parse("gomod:std:fmt")
	if err != nil {
		t.Fatal(err)
	}
	if r.Realm != "gomod" || r.Path != "std:fmt" {
		t.Fatalf("got %+v", r)
	}
}

func TestParseSymbol(t *testing.T) {
	r, err := Parse("gobind:desktop/app.go#App.Submit")
	if err != nil {
		t.Fatal(err)
	}
	if r.Realm != "gobind" || r.Path != "desktop/app.go" || r.Symbol != "App.Submit" {
		t.Fatalf("got %+v", r)
	}
}

func TestParseLineAndSymbol(t *testing.T) {
	r, err := Parse("bridge:desktop/frontend/src/lib/bridge.ts:142#app.Submit")
	if err != nil {
		t.Fatal(err)
	}
	if r.Realm != "bridge" || r.Path != "desktop/frontend/src/lib/bridge.ts" || r.Line != 142 || r.Symbol != "app.Submit" {
		t.Fatalf("got %+v", r)
	}
	if got := NewWithLine("bridge", r.Path, r.Line, r.Symbol); got != r.String() {
		t.Fatalf("NewWithLine = %q, want %q", got, r.String())
	}
}

func TestNew(t *testing.T) {
	if got := New("js", "desktop/frontend/src/lib", ""); got != "js:desktop/frontend/src/lib" {
		t.Fatalf("New = %q", got)
	}
	if got := New("hook", "desktop/frontend/src/lib/useX.ts", "useX"); got != "hook:desktop/frontend/src/lib/useX.ts#useX" {
		t.Fatalf("New with symbol = %q", got)
	}
}

func TestParseErrors(t *testing.T) {
	for _, id := range []string{"", "nocolon", "go:", "go:path#", ":path"} {
		if _, err := Parse(id); err == nil {
			t.Fatalf("Parse(%q) want error", id)
		}
	}
}

func TestMustParse(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	MustParse("")
}

func TestRoundTrip(t *testing.T) {
	cases := []string{
		"go:arcdesk/internal/agent",
		"gomod:std:fmt",
		"npm:react",
		"gobind:desktop/app.go#App.ListTabs",
		"bridge:desktop/frontend/src/Composer.tsx:891#app.Submit",
	}
	for _, id := range cases {
		r, err := Parse(id)
		if err != nil {
			t.Fatalf("Parse(%q): %v", id, err)
		}
		if r.String() != id {
			t.Fatalf("round trip %q -> %q", id, r.String())
		}
	}
}
