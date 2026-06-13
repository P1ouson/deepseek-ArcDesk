package selfdebug

import (
	"fmt"
	"strings"

	"arcdesk/internal/callgraph"
	"arcdesk/internal/dependency"
	"arcdesk/internal/runtime"
	"arcdesk/internal/verification"
)

// IsVerifyCommand reports whether cmd looks like a verification command.
func IsVerifyCommand(cmd string) bool {
	return verification.IsVerifyCommand(cmd)
}

// BuildImmediateHint returns a short directive appended to a failed bash result.
func BuildImmediateHint(in Input) string {
	if !in.HasWriter {
		return ""
	}
	failedCmd := resolveFailedCmd(in)
	if failedCmd == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("[self-debug] Verification failed for `")
	b.WriteString(failedCmd)
	b.WriteString("`. Fix the errors above, then re-run that command.")
	if pending := pendingCommandsExcluding(in, failedCmd); len(pending) > 0 {
		b.WriteString(" Still required after writes:")
		for _, p := range pending {
			b.WriteString("\n- ")
			b.WriteString(p)
		}
	}
	return b.String()
}

// BuildRetryContext returns the unified self-debug block for readiness retries.
func BuildRetryContext(in Input) string {
	if !in.HasWriter {
		return ""
	}
	failedCmd := resolveFailedCmd(in)
	pending := pendingCommands(in)
	if failedCmd == "" && len(pending) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Self-debug Loop\n")
	b.WriteString("Cycle: **write → verify → analyze → fix → re-verify**\n")
	if in.Attempt > 0 && in.MaxRetries > 0 {
		b.WriteString(fmt.Sprintf("Attempt %d/%d\n", in.Attempt, in.MaxRetries))
	}
	if failedCmd != "" {
		b.WriteString("- failed check: ")
		b.WriteString(failedCmd)
		b.WriteByte('\n')
	}
	if paths := limitPaths(in.WrittenPaths, 8); len(paths) > 0 {
		b.WriteString("- changed: ")
		b.WriteString(strings.Join(paths, ", "))
		b.WriteByte('\n')
	}
	if len(pending) > 0 {
		b.WriteString("- still required:\n")
		for _, p := range pending {
			b.WriteString("  - ")
			b.WriteString(p)
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n### Next\n")
	b.WriteString("1. Read the failure output and runtime signals below\n")
	b.WriteString("2. Edit the changed files to fix the root cause\n")
	if failedCmd != "" {
		b.WriteString("3. Re-run: ")
		b.WriteString(failedCmd)
		b.WriteByte('\n')
	}
	b.WriteString("4. Do not give a final answer until every required check succeeds\n")

	if in.DepIndex != nil && len(in.WrittenPaths) > 0 {
		if block := dependency.BuildFailureContext(in.DepIndex, in.WrittenPaths, failedCmd, in.Stderr); block != "" {
			b.WriteString("\n")
			b.WriteString(block)
			b.WriteByte('\n')
		}
	}
	cgPaths := in.CallgraphPaths
	if len(cgPaths) == 0 {
		cgPaths = in.WrittenPaths
	}
	if in.CGIndex != nil && len(cgPaths) > 0 {
		if block := callgraph.BuildCrossRealmContext(in.CGIndex, cgPaths, failedCmd); block != "" {
			b.WriteString("\n")
			b.WriteString(block)
			b.WriteByte('\n')
		}
	}
	if in.RuntimeHub != nil {
		if block := runtime.BuildVerifyContext(in.RuntimeHub, failedCmd, in.Stderr); block != "" {
			b.WriteString("\n")
			b.WriteString(block)
			b.WriteByte('\n')
		}
	}
	if len(in.Checks) > 0 {
		if block := verification.BuildRetryContext(in.Checks, failedCmd, in.Stderr); block != "" {
			b.WriteString("\n")
			b.WriteString(block)
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

// AnalyzeSnapshot derives tool-visible loop state from input.
func AnalyzeSnapshot(in Input) Snapshot {
	explicitFailed := strings.TrimSpace(in.FailedCmd)
	pending := pendingCommands(in)
	passed := passedCommands(in)
	phase := PhaseIdle
	if explicitFailed != "" {
		phase = PhaseFailed
	} else if len(pending) > 0 {
		phase = PhaseVerify
	}
	failedCmd := explicitFailed
	if failedCmd == "" {
		failedCmd = resolveFailedCmd(in)
	}
	return Snapshot{
		Phase:         phase,
		Attempt:       in.Attempt,
		MaxRetries:    in.MaxRetries,
		FailedCmd:     failedCmd,
		PendingChecks: pending,
		PassedChecks:  passed,
		WrittenPaths:  limitPaths(in.WrittenPaths, 12),
	}
}

func resolveFailedCmd(in Input) string {
	if cmd := strings.TrimSpace(in.FailedCmd); cmd != "" {
		return cmd
	}
	for _, c := range in.Checks {
		cmd := strings.TrimSpace(c.Command)
		if cmd == "" {
			continue
		}
		if in.HasSuccessfulCommandAfter != nil && !in.HasSuccessfulCommandAfter(cmd, in.WriterIndex) && IsVerifyCommand(cmd) {
			return cmd
		}
	}
	return ""
}

func pendingCommandsExcluding(in Input, exclude string) []string {
	pending := pendingCommands(in)
	if exclude == "" {
		return pending
	}
	out := pending[:0]
	for _, p := range pending {
		if p != exclude {
			out = append(out, p)
		}
	}
	return out
}

func pendingCommands(in Input) []string {
	if in.HasSuccessfulCommandAfter == nil {
		return nil
	}
	var out []string
	for _, c := range in.Checks {
		cmd := strings.TrimSpace(c.Command)
		if cmd == "" || in.HasSuccessfulCommandAfter(cmd, in.WriterIndex) {
			continue
		}
		out = append(out, cmd)
	}
	return out
}

func passedCommands(in Input) []string {
	if in.HasSuccessfulCommandAfter == nil {
		return nil
	}
	var out []string
	for _, c := range in.Checks {
		cmd := strings.TrimSpace(c.Command)
		if cmd == "" || !in.HasSuccessfulCommandAfter(cmd, in.WriterIndex) {
			continue
		}
		out = append(out, cmd)
	}
	return out
}

func limitPaths(paths []string, max int) []string {
	if max <= 0 || len(paths) <= max {
		return append([]string(nil), paths...)
	}
	return append([]string(nil), paths[:max]...)
}
