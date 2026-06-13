package runtime

import (
	"fmt"
	"strings"
	"time"
)

var verifyKeywords = []string{"build", "test", "vet", "lint", "compile", "pnpm", "npm", "vitest", "playwright"}

// IsVerifyCommand reports whether cmd looks like a verification command.
func IsVerifyCommand(cmd string) bool {
	cmd = strings.ToLower(strings.TrimSpace(cmd))
	if cmd == "" {
		return false
	}
	for _, kw := range verifyKeywords {
		if strings.Contains(cmd, kw) {
			return true
		}
	}
	return false
}

// BuildVerifyContext formats recent runtime observations for verify retries.
func BuildVerifyContext(hub *Hub, failedCmd, stderr string) string {
	if hub == nil || !IsVerifyCommand(failedCmd) {
		return ""
	}
	since := time.Now().UTC().Add(-15 * time.Minute)
	var b strings.Builder
	b.WriteString("## Runtime Observation\n")

	wrote := false
	if block := formatTail(hub, TailQuery{Kind: KindConsole, Since: since, Limit: 8, Errors: true}); block != "" {
		b.WriteString("### Console\n")
		b.WriteString(block)
		wrote = true
	}
	if block := formatTail(hub, TailQuery{Kind: KindNetwork, Since: since, Limit: 8, Errors: true}); block != "" {
		b.WriteString("### Network\n")
		b.WriteString(block)
		wrote = true
	}
	if block := formatTail(hub, TailQuery{Kind: KindGoLog, Since: since, Limit: 10, Errors: true}); block != "" {
		b.WriteString("### Process output\n")
		b.WriteString(block)
		wrote = true
	}
	if block := formatTail(hub, TailQuery{Kind: KindWails, Since: since, Limit: 10, Errors: true}); block != "" {
		b.WriteString("### Wails / desktop\n")
		b.WriteString(block)
		wrote = true
	}
	if !wrote {
		if s := strings.TrimSpace(stderr); s != "" {
			b.WriteString("### Process output\n")
			b.WriteString(truncate(s, 1500))
			b.WriteByte('\n')
			wrote = true
		}
	}
	if lines := FormatStateLines(hub.State()); len(lines) > 0 {
		b.WriteString("### Runtime state\n")
		for i, ln := range lines {
			if i >= 12 {
				b.WriteString("…\n")
				break
			}
			b.WriteString(ln)
			b.WriteByte('\n')
		}
		wrote = true
	}
	if !wrote {
		return ""
	}
	return strings.TrimSpace(b.String())
}

func formatTail(hub *Hub, q TailQuery) string {
	entries := hub.Tail(q)
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for i, e := range entries {
		if i >= q.Limit && q.Limit > 0 {
			break
		}
		line := strings.TrimSpace(e.Message)
		if line == "" {
			continue
		}
		if e.Meta != nil {
			if url := e.Meta["url"]; url != "" {
				line = fmt.Sprintf("%s (%s)", line, url)
			}
			if st := e.Meta["status"]; st != "" {
				line = fmt.Sprintf("[%s] %s", st, line)
			}
		}
		b.WriteString("- ")
		b.WriteString(truncate(line, 300))
		b.WriteByte('\n')
	}
	return b.String()
}
