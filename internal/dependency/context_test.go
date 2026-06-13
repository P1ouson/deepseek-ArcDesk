package dependency

import (
	"context"
	"strings"
	"testing"
)

func TestIsVerifyCommand(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"go test ./...", true},
		{"npm run build", true},
		{"golangci-lint run", true},
		{"go vet ./...", true},
		{"make compile-all", true},
		{"git status", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsVerifyCommand(tc.cmd); got != tc.want {
			t.Fatalf("IsVerifyCommand(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

func TestBuildFailureContextGoProject(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	got := BuildFailureContext(idx, []string{"internal/alpha/alpha.go"}, "go test ./...", "FAIL example.com/alpha")
	if got == "" {
		t.Fatal("expected non-empty context")
	}
	if !strings.Contains(got, "## Dependency Impact") {
		t.Fatalf("missing heading: %q", got)
	}
	if strings.Count(got, "\n")+1 > 8 {
		t.Fatalf("expected <= 8 lines, got %d:\n%s", strings.Count(got, "\n")+1, got)
	}
	if !strings.Contains(got, "go test ./...") {
		t.Fatalf("missing failed command: %q", got)
	}
}

func TestBuildFailureContextSkipsNonVerify(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := BuildFailureContext(idx, []string{"a.go"}, "git diff", "oops"); got != "" {
		t.Fatalf("BuildFailureContext() = %q, want empty", got)
	}
}

func TestBuildFailureContextNilIndex(t *testing.T) {
	got := BuildFailureContext(nil, nil, "go test", "fail")
	if got != "dependency index unavailable" {
		t.Fatalf("got %q", got)
	}
}

func TestRiskBeforeEditPhase1Empty(t *testing.T) {
	if got := RiskBeforeEdit(nil, []string{"a.go"}); got != "" {
		t.Fatalf("RiskBeforeEdit() = %q, want empty", got)
	}
}
