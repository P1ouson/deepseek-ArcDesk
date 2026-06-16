package verifyselect

import (
	"testing"

	"arcdesk/internal/verification"
)

func TestMinimumChecksDocsOnlyDropsE2E(t *testing.T) {
	plan := verification.Plan{
		Checks: []verification.Check{
			{Command: "npm run build", Category: verification.CategoryBuild},
			{Command: "npm run e2e", Category: verification.CategoryE2E},
		},
	}
	out := MinimumChecks(plan, []string{"CHANGELOG.md", "docs/readme.md"})
	for _, c := range out.Checks {
		if c.Category == verification.CategoryE2E {
			t.Fatalf("e2e should be dropped for docs-only: %+v", out.Checks)
		}
	}
}

func TestMinimumChecksStyleOnly(t *testing.T) {
	plan := verification.Plan{
		Checks: []verification.Check{
			{Command: "npm run build", Category: verification.CategoryBuild},
			{Command: "npm run test", Category: verification.CategoryUnit},
			{Command: "npm run e2e", Category: verification.CategoryE2E},
		},
	}
	out := MinimumChecks(plan, []string{"desktop/frontend/src/App.css"})
	for _, c := range out.Checks {
		if c.Category == verification.CategoryE2E || c.Category == verification.CategoryUnit {
			t.Fatalf("unexpected check %v for css-only", c)
		}
	}
}

func TestMinimumChecksEmptyPathsKeepsAll(t *testing.T) {
	plan := verification.Plan{
		Checks: []verification.Check{{Command: "go test ./...", Category: verification.CategoryUnit}},
	}
	out := MinimumChecks(plan, nil)
	if len(out.Checks) != 1 {
		t.Fatalf("checks = %d", len(out.Checks))
	}
}
