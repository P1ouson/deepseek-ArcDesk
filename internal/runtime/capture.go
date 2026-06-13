package runtime

import (
	"io"
	"strings"
	"sync"

	"arcdesk/internal/event"
)

// CaptureSink wraps an inner sink, recording agent-visible runtime signals into hub.
type CaptureSink struct {
	Inner event.Sink
	Hub   *Hub
}

// NewCaptureSink returns a sink that forwards to inner and taps hub.
func NewCaptureSink(inner event.Sink, hub *Hub) *CaptureSink {
	if inner == nil {
		inner = event.Discard
	}
	return &CaptureSink{Inner: inner, Hub: hub}
}

func (s *CaptureSink) Emit(e event.Event) {
	if s == nil {
		return
	}
	if s.Hub != nil {
		s.tap(e)
	}
	if s.Inner != nil {
		s.Inner.Emit(e)
	}
}

func (s *CaptureSink) tap(e event.Event) {
	switch e.Kind {
	case event.TurnStarted:
		// Turn is stamped by the controller separately via Hub.SetTurn; keep a
		// best-effort marker when only the agent sink is wired.
		s.Hub.Ingest(KindState, LevelInfo, "agent", "turn_started", map[string]string{"event": "turn_started"})
	case event.Notice:
		level := LevelInfo
		switch e.Level {
		case event.LevelWarn:
			level = LevelWarn
		}
		if strings.TrimSpace(e.Text) != "" {
			s.Hub.Ingest(KindGoLog, level, "notice", e.Text, nil)
		}
	case event.ToolProgress:
		if isShellTool(e.Tool.Name) && strings.TrimSpace(e.Tool.Output) != "" {
			s.Hub.Ingest(KindGoLog, LevelInfo, e.Tool.Name, e.Tool.Output, map[string]string{
				"tool_id": e.Tool.ID,
				"partial": "true",
			})
		}
	case event.ToolResult:
		t := e.Tool
		meta := map[string]string{"tool": t.Name, "tool_id": t.ID}
		if t.Err != "" {
			meta["error"] = truncate(t.Err, 500)
			s.Hub.Ingest(KindGoLog, LevelError, t.Name, t.Err, meta)
		}
		if isShellTool(t.Name) {
			out := strings.TrimSpace(firstNonEmpty(t.Output, t.Err))
			if out != "" {
				level := LevelInfo
				if t.Err != "" {
					level = LevelError
				}
				s.Hub.Ingest(KindGoLog, level, t.Name, out, meta)
			}
		}
	}
}

func isShellTool(name string) bool {
	switch name {
	case "bash", "run_shell", "run_terminal_cmd":
		return true
	default:
		return false
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// StderrWriter mirrors plugin/diagnostic stderr into the hub.
type StderrWriter struct {
	Inner io.Writer
	Hub   *Hub
	buf   strings.Builder
	mu    sync.Mutex
}

// NewStderrWriter wraps w, forwarding bytes and recording complete lines.
func NewStderrWriter(w io.Writer, hub *Hub) *StderrWriter {
	if w == nil {
		w = io.Discard
	}
	return &StderrWriter{Inner: w, Hub: hub}
}

func (sw *StderrWriter) Write(p []byte) (int, error) {
	if sw == nil {
		return len(p), nil
	}
	n, err := sw.Inner.Write(p)
	if sw.Hub != nil && len(p) > 0 {
		sw.mu.Lock()
		sw.buf.Write(p)
		for {
			line, ok := nextLine(&sw.buf)
			if !ok {
				break
			}
			line = strings.TrimSpace(line)
			if line != "" {
				sw.Hub.Ingest(KindGoLog, LevelWarn, "stderr", line, map[string]string{"stream": "stderr"})
			}
		}
		sw.mu.Unlock()
	}
	return n, err
}

func nextLine(b *strings.Builder) (string, bool) {
	s := b.String()
	i := strings.IndexByte(s, '\n')
	if i < 0 {
		return "", false
	}
	line := s[:i]
	b.Reset()
	b.WriteString(s[i+1:])
	return line, true
}

// IngestBatch ingests desktop/frontend payloads.
type IngestItem struct {
	Kind    Kind              `json:"kind"`
	Level   Level             `json:"level"`
	Source  string            `json:"source"`
	Message string            `json:"message"`
	Meta    map[string]string `json:"meta"`
}

// IngestBatch records multiple observations atomically.
func (h *Hub) IngestBatch(items []IngestItem) {
	if h == nil {
		return
	}
	for _, it := range items {
		h.Ingest(it.Kind, it.Level, it.Source, it.Message, it.Meta)
	}
}
