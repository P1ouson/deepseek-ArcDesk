package ctxcompress

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const defaultMaxToolOutputBytes = 32 * 1024

// Config governs tool-output compression before it enters model context.
type Config struct {
	Enabled            bool
	ToolOutputMaxBytes int
}

// Compressor applies byte limits to large tool results.
type Compressor struct {
	cfg Config
}

// New returns a compressor. Zero max bytes falls back to 32 KiB when enabled.
func New(cfg Config) *Compressor {
	return &Compressor{cfg: cfg}
}

// Enabled reports whether compression overrides are active.
func (c *Compressor) Enabled() bool {
	return c != nil && c.cfg.Enabled
}

// MaxToolOutputBytes returns the configured cap (0 when disabled).
func (c *Compressor) MaxToolOutputBytes() int {
	if c == nil || !c.cfg.Enabled {
		return 0
	}
	max := c.cfg.ToolOutputMaxBytes
	if max <= 0 {
		max = defaultMaxToolOutputBytes
	}
	return max
}

// Status is returned by context_compression_status.
type Status struct {
	Enabled            bool `json:"enabled"`
	ToolOutputMaxBytes int  `json:"tool_output_max_bytes"`
	DefaultMaxBytes    int  `json:"default_max_bytes"`
}

// Snapshot reports current settings.
func (c *Compressor) Snapshot() Status {
	if c == nil {
		return Status{DefaultMaxBytes: defaultMaxToolOutputBytes}
	}
	return Status{
		Enabled:            c.cfg.Enabled,
		ToolOutputMaxBytes: c.MaxToolOutputBytes(),
		DefaultMaxBytes:    defaultMaxToolOutputBytes,
	}
}

// Truncate head+tails s when it exceeds maxBytes.
func Truncate(s string, maxBytes int) (body, notice string) {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s, ""
	}
	keep := maxBytes / 2
	head := snapToRuneBoundary(s, 0, keep)
	tail := snapToRuneBoundary(s, len(s)-keep, len(s))
	omitted := len(s) - len(head) - len(tail)
	notice = fmt.Sprintf("tool output compressed: %d of %d bytes elided", omitted, len(s))
	body = head + fmt.Sprintf("\n\n…[compressed %d of %d bytes — use read_file offset/limit]…\n\n", omitted, len(s)) + tail
	return body, notice
}

// DigestLines keeps matching lines plus head/tail context for log-like output.
func DigestLines(s, query string, maxLines, context int) string {
	query = strings.ToLower(strings.TrimSpace(query))
	lines := strings.Split(s, "\n")
	if query == "" || len(lines) <= maxLines {
		return s
	}
	type span struct{ lo, hi int }
	var hits []span
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			lo := i - context
			if lo < 0 {
				lo = 0
			}
			hi := i + context
			if hi >= len(lines) {
				hi = len(lines) - 1
			}
			hits = append(hits, span{lo, hi})
		}
	}
	if len(hits) == 0 {
		return s
	}
	// Merge spans
	merged := []span{hits[0]}
	for _, h := range hits[1:] {
		last := &merged[len(merged)-1]
		if h.lo <= last.hi+1 {
			if h.hi > last.hi {
				last.hi = h.hi
			}
			continue
		}
		merged = append(merged, h)
	}
	var b strings.Builder
	prevEnd := -1
	for _, m := range merged {
		if prevEnd >= 0 && m.lo > prevEnd+1 {
			fmt.Fprintf(&b, "\n…[%d lines omitted]…\n", m.lo-prevEnd-1)
		}
		for i := m.lo; i <= m.hi; i++ {
			if i < len(lines) {
				b.WriteString(lines[i])
				b.WriteByte('\n')
			}
		}
		prevEnd = m.hi
	}
	return strings.TrimRight(b.String(), "\n")
}

func snapToRuneBoundary(s string, lo, hi int) string {
	for lo > 0 && !utf8.RuneStart(s[lo]) {
		lo--
	}
	for hi < len(s) && !utf8.RuneStart(s[hi]) {
		hi++
	}
	return s[lo:hi]
}
