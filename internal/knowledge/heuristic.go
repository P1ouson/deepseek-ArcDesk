package knowledge

import (
	"fmt"
	"strings"

	"arcdesk/internal/failuremem"
)

func heuristicSourceFixProposal(failedCmd, errOut string, paths []string) (CaptureProposal, bool) {
	if !pathsIncludeSourceFix(paths) {
		return CaptureProposal{}, false
	}
	srcPaths := sourceFixPaths(paths)
	if len(srcPaths) == 0 {
		return CaptureProposal{}, false
	}
	sig := strings.TrimSpace(failedCmd)
	if sig == "" {
		sig = "verification failure"
	}
	errText := truncateField(strings.TrimSpace(errOut), 500)
	files := strings.Join(srcPaths, ", ")
	fix := fmt.Sprintf(
		"Fix the implementation bug in %s that caused the verify failure (check return values and state updates), then re-run %s.",
		files, sig,
	)
	summary := fmt.Sprintf("Fix source logic in %s", srcPaths[0])
	j := CaptureJudgment{
		Record:    true,
		Signature: sig,
		Error:     errText,
		Fix:       fix,
		Summary:   summary,
	}
	if !failuremem.QualifiesForRecord(failuremem.Entry{Signature: j.Signature, Fix: j.Fix}) {
		return CaptureProposal{}, false
	}
	return proposalFromJudgment(j, paths)
}

func sourceFixPaths(paths []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range paths {
		p = strings.TrimSpace(strings.ReplaceAll(p, `\`, "/"))
		if p == "" {
			continue
		}
		base := p
		if i := strings.LastIndex(base, "/"); i >= 0 {
			base = base[i+1:]
		}
		low := strings.ToLower(base)
		if strings.HasSuffix(low, "_test.go") || strings.HasSuffix(low, ".test.ts") || strings.HasSuffix(low, ".test.tsx") {
			continue
		}
		if strings.HasSuffix(low, ".go") || strings.HasSuffix(low, ".ts") || strings.HasSuffix(low, ".tsx") {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}
