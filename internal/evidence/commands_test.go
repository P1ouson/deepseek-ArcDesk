package evidence

import "testing"

func TestCommandSatisfiesCheckDogfoodCases(t *testing.T) {
	root := `C:\Users\dell\Desktop\p0-dogfood-demo`
	cases := []struct {
		executed string
		required string
		want     bool
	}{
		{"go test ./...", "go test ./...", true},
		{root + `; go test ./...`, "go test ./...", true},
		{"cd " + root + "; go test ./...", "go test ./...", true},
		{"go clean -testcache; go test ./...", "go test ./...", true},
		{"go test ./internal/counter/ -v", "go test ./...", true},
		{"go test ./internal/counter/...", "go test ./...", true},
		{"go build ./...", "go build ./...", true},
		{"cd " + root + "; go build ./...", "go build ./...", true},
		{"go vet ./...", "go vet ./...", true},
		{"npm run build --prefix desktop/frontend", "npm run build --prefix desktop/frontend", true},
		{"cd " + root + "; npm run build --prefix desktop/frontend", "npm run build --prefix desktop/frontend", true},
		{"go test ./internal/...", "go test ./...", true},
		{"echo hello", "go test ./...", false},
		{"go test ./... 2>&1", "go test ./...", true},
	}
	for _, tc := range cases {
		got := CommandSatisfiesCheck(tc.executed, tc.required)
		if got != tc.want {
			t.Fatalf("CommandSatisfiesCheck(%q, %q) = %v, want %v", tc.executed, tc.required, got, tc.want)
		}
	}
}

func TestHasSuccessfulCommandAfterCompoundShell(t *testing.T) {
	ledger := NewLedger()
	ledger.Record(Receipt{ToolName: "write_file", Success: true, Paths: []string{"counter.go"}, Write: true})
	writer, _ := ledger.LatestSuccessfulWriterIndex()
	ledger.Record(Receipt{
		ToolName: "bash",
		Success:  true,
		Command:  `cd C:\demo; go build ./...`,
	})
	ledger.Record(Receipt{
		ToolName: "bash",
		Success:  true,
		Command:  `go clean -testcache; go test ./...`,
	})
	if !ledger.HasSuccessfulCommandAfter("go build ./...", writer) {
		t.Fatal("compound go build should satisfy check")
	}
	if !ledger.HasSuccessfulCommandAfter("go test ./...", writer) {
		t.Fatal("compound go test should satisfy check")
	}
}
