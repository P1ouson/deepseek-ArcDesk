package ctxcompress

import "testing"

func TestTruncate(t *testing.T) {
	s := string(make([]byte, 1000))
	body, notice := Truncate(s, 200)
	if notice == "" || len(body) >= len(s) {
		t.Fatalf("notice=%q len(body)=%d", notice, len(body))
	}
}

func TestCompressorMaxBytes(t *testing.T) {
	c := New(Config{Enabled: true, ToolOutputMaxBytes: 8192})
	if c.MaxToolOutputBytes() != 8192 {
		t.Fatalf("max = %d", c.MaxToolOutputBytes())
	}
}
