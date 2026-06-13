package ctxcompress

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestCompressorDisabledAndDefaults(t *testing.T) {
	var c *Compressor
	if c.Enabled() || c.MaxToolOutputBytes() != 0 {
		t.Fatal("nil compressor")
	}
	snap := c.Snapshot()
	if snap.DefaultMaxBytes != defaultMaxToolOutputBytes {
		t.Fatal("default bytes in snap")
	}
	off := New(Config{Enabled: false})
	if off.MaxToolOutputBytes() != 0 {
		t.Fatal("disabled max")
	}
	on := New(Config{Enabled: true})
	if on.MaxToolOutputBytes() != defaultMaxToolOutputBytes {
		t.Fatalf("enabled default max = %d", on.MaxToolOutputBytes())
	}
}

func TestTruncateNoOp(t *testing.T) {
	body, notice := Truncate("short", 100)
	if body != "short" || notice != "" {
		t.Fatalf("body=%q notice=%q", body, notice)
	}
	body, notice = Truncate("x", 0)
	if body != "x" || notice != "" {
		t.Fatalf("zero max: body=%q notice=%q", body, notice)
	}
}

func TestDigestLines(t *testing.T) {
	lines := strings.Repeat("line\n", 20)
	src := lines + "needle here\n" + lines
	out := DigestLines(src, "needle", 5, 1)
	if !strings.Contains(out, "needle here") {
		t.Fatalf("digest=%q", out)
	}
	if DigestLines("abc", "", 1, 0) != "abc" {
		t.Fatal("empty query passthrough")
	}
	if DigestLines("abc", "z", 1, 0) != "abc" {
		t.Fatal("no hits passthrough")
	}
}

func TestCtxCompressToolsExecute(t *testing.T) {
	c := New(Config{Enabled: true, ToolOutputMaxBytes: 4096})
	reg := tool.NewRegistry()
	RegisterTools(reg, c)
	status, ok := reg.Get("context_compression_status")
	if !ok {
		t.Fatal("missing tool")
	}
	if status.Name() == "" || status.Description() == "" || !status.ReadOnly() {
		t.Fatal("tool meta")
	}
	out, err := status.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "4096") {
		t.Fatalf("status=%q err=%v", out, err)
	}
}

func TestDigestLinesMergedSpans(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	b.WriteString("hit one\n")
	for i := 0; i < 5; i++ {
		fmt.Fprintf(&b, "mid %d\n", i)
	}
	b.WriteString("hit two\n")
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&b, "tail %d\n", i)
	}
	out := DigestLines(b.String(), "hit", 3, 0)
	if !strings.Contains(out, "hit one") || !strings.Contains(out, "hit two") || !strings.Contains(out, "omitted") {
		t.Fatalf("digest=%q", out)
	}
}

func TestTruncateMultibyteBoundary(t *testing.T) {
	s := strings.Repeat("日", 500)
	body, notice := Truncate(s, 100)
	if notice == "" || len(body) >= len(s) {
		t.Fatalf("truncate multibyte len=%d notice=%q", len(body), notice)
	}
}

func TestCtxCompressRegisterNilSafe(t *testing.T) {
	RegisterTools(nil, New(Config{Enabled: true}))
	RegisterTools(tool.NewRegistry(), nil)
}
