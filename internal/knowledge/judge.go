package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"arcdesk/internal/failuremem"
	"arcdesk/internal/nilutil"
	"arcdesk/internal/provider"
)

const captureJudgePrompt = `You decide whether a coding workspace should save a failure→fix lesson to long-term memory after verification later passed.

Return ONLY JSON:
{"record":true|false,"signature":"short label","error":"what failed (1-2 sentences)","fix":"what fixed it — concrete and reusable (1-3 sentences)","summary":"one UI line","reason":"if record is false, why skip; else empty string"}

Record TRUE when the fix teaches something reusable:
- logic/return bugs in implementation source (*.go but not *_test.go)
- undefined symbols, missing imports, compile errors, wrong types, config/build mistakes
- test failures caused by incorrect production code that was corrected in source files

Record FALSE only for low-value noise:
- ONLY the test file changed and the lesson is just flipping expected literals (e.g. want 5 → 99 → 5)
- formatter-only edits, empty restatements, or "re-run until green" with no actionable fix

If SOURCE_FIX=yes (implementation file edited, not just tests), and the failure output shows tests failing because code was wrong, prefer record=true with a concrete fix sentence.

Write fix/summary in the same language as the failure output when obvious (Chinese output → Chinese fix); otherwise English.
Do not say only "edited file X" — explain what was wrong and what to change.`

// CaptureJudgeInput is the context passed to a capture judger.
type CaptureJudgeInput struct {
	FailedCmd string
	ErrOut    string
	Paths     []string
}

// CaptureJudgment is the structured result from JudgeCapture.
type CaptureJudgment struct {
	Record    bool
	Signature string
	Error     string
	Fix       string
	Summary   string
	Reason    string
}

// CaptureJudger decides whether a verify fail→pass lesson belongs in memory.
type CaptureJudger interface {
	JudgeCapture(ctx context.Context, in CaptureJudgeInput) (CaptureJudgment, error)
}

// FuncCaptureJudger adapts a function to CaptureJudger.
type FuncCaptureJudger func(ctx context.Context, in CaptureJudgeInput) (CaptureJudgment, error)

func (f FuncCaptureJudger) JudgeCapture(ctx context.Context, in CaptureJudgeInput) (CaptureJudgment, error) {
	if f == nil {
		return CaptureJudgment{}, fmt.Errorf("capture judger not configured")
	}
	return f(ctx, in)
}

// HeuristicCaptureJudger records lessons using local rules only (no LLM).
type HeuristicCaptureJudger struct{}

func NewHeuristicCaptureJudger() *HeuristicCaptureJudger {
	return &HeuristicCaptureJudger{}
}

func (j *HeuristicCaptureJudger) JudgeCapture(_ context.Context, in CaptureJudgeInput) (CaptureJudgment, error) {
	proposal, ok := heuristicSourceFixProposal(in.FailedCmd, in.ErrOut, in.Paths)
	if !ok {
		return CaptureJudgment{Record: false, Reason: "no heuristic match"}, nil
	}
	return CaptureJudgment{
		Record:    true,
		Signature: proposal.Signature,
		Error:     proposal.Error,
		Fix:       proposal.Fix,
		Summary:   proposal.Summary,
	}, nil
}

// ProviderCaptureJudger uses the workspace model to judge capture worthiness.
type ProviderCaptureJudger struct {
	prov provider.Provider
}

func NewProviderCaptureJudger(prov provider.Provider) *ProviderCaptureJudger {
	if nilutil.IsNil(prov) {
		return nil
	}
	return &ProviderCaptureJudger{prov: prov}
}

func (j *ProviderCaptureJudger) JudgeCapture(ctx context.Context, in CaptureJudgeInput) (CaptureJudgment, error) {
	if j == nil || nilutil.IsNil(j.prov) {
		return CaptureJudgment{}, fmt.Errorf("capture judge provider not configured")
	}
	user := formatCaptureJudgeUser(in)
	ch, err := j.prov.Stream(ctx, provider.Request{
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: captureJudgePrompt},
			{Role: provider.RoleUser, Content: user},
		},
		Temperature: 0,
		MaxTokens:   320,
	})
	if err != nil {
		return CaptureJudgment{}, err
	}

	var text strings.Builder
	for chunk := range ch {
		switch chunk.Type {
		case provider.ChunkText:
			text.WriteString(chunk.Text)
		case provider.ChunkError:
			return CaptureJudgment{}, chunk.Err
		}
	}
	return parseCaptureJudgment(text.String(), in)
}

func formatCaptureJudgeUser(in CaptureJudgeInput) string {
	errOut := strings.TrimSpace(in.ErrOut)
	if len(errOut) > 1800 {
		errOut = errOut[:1797] + "..."
	}
	var b strings.Builder
	b.WriteString("VERIFY_COMMAND:\n")
	b.WriteString(strings.TrimSpace(in.FailedCmd))
	b.WriteString("\n\nFAILURE_OUTPUT:\n")
	b.WriteString(errOut)
	if len(in.Paths) > 0 {
		b.WriteString("\n\nEDITED_PATHS:\n")
		b.WriteString(strings.Join(in.Paths, "\n"))
	}
	if sourceFix := pathsIncludeSourceFix(in.Paths); sourceFix {
		b.WriteString("\n\nSOURCE_FIX: yes (implementation file edited)\n")
	} else {
		b.WriteString("\n\nSOURCE_FIX: no\n")
	}
	return b.String()
}

func pathsIncludeSourceFix(paths []string) bool {
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
			return true
		}
	}
	return false
}

func parseCaptureJudgment(raw string, in CaptureJudgeInput) (CaptureJudgment, error) {
	var out struct {
		Record    *bool  `json:"record"`
		Signature string `json:"signature"`
		Error     string `json:"error"`
		Fix       string `json:"fix"`
		Summary   string `json:"summary"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(raw)), &out); err != nil {
		return CaptureJudgment{}, fmt.Errorf("decode capture judgment: %w", err)
	}
	if out.Record == nil {
		return CaptureJudgment{}, fmt.Errorf("decode capture judgment: missing record")
	}
	j := CaptureJudgment{
		Record:    *out.Record,
		Signature: strings.TrimSpace(out.Signature),
		Error:     strings.TrimSpace(out.Error),
		Fix:       strings.TrimSpace(out.Fix),
		Summary:   strings.TrimSpace(out.Summary),
		Reason:    strings.TrimSpace(out.Reason),
	}
	if j.Signature == "" {
		j.Signature = strings.TrimSpace(in.FailedCmd)
	}
	if j.Error == "" {
		j.Error = truncateField(strings.TrimSpace(in.ErrOut), 500)
	}
	if j.Record && j.Fix == "" {
		return CaptureJudgment{}, fmt.Errorf("decode capture judgment: record=true but fix empty")
	}
	if j.Record && j.Summary == "" {
		j.Summary = j.Fix
	}
	if !j.Record {
		return j, nil
	}
	if !failuremem.QualifiesForRecord(failuremem.Entry{Signature: j.Signature, Fix: j.Fix}) {
		j.Record = false
		j.Reason = "fix too short or generic"
	}
	return j, nil
}

func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start >= 0 && end >= start {
		return s[start : end+1]
	}
	return s
}

func proposalFromJudgment(j CaptureJudgment, paths []string) (CaptureProposal, bool) {
	if !j.Record {
		return CaptureProposal{}, false
	}
	sig := strings.TrimSpace(j.Signature)
	fix := strings.TrimSpace(j.Fix)
	errOut := strings.TrimSpace(j.Error)
	if sig == "" || fix == "" {
		return CaptureProposal{}, false
	}
	if len(errOut) > 500 {
		errOut = errOut[:497] + "..."
	}
	entry := failuremem.Entry{
		Signature: sig,
		Error:     errOut,
		Fix:       fix,
		Paths:     append([]string(nil), paths...),
		Kind:      failuremem.KindFix,
	}
	failuremem.NormalizeEntry(&entry)
	summary := strings.TrimSpace(j.Summary)
	if summary == "" {
		summary = fix
	}
	return CaptureProposal{
		ID:          entry.ID,
		Fingerprint: failuremem.Fingerprint(entry),
		Signature:   sig,
		Summary:     summary,
		Error:       errOut,
		Fix:         fix,
		Paths:       append([]string(nil), paths...),
	}, true
}
