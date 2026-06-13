package runtime

import "time"

// Kind classifies a runtime observation record.
type Kind string

const (
	KindConsole Kind = "console"
	KindGoLog   Kind = "go_log"
	KindWails   Kind = "wails"
	KindNetwork Kind = "network"
	KindState   Kind = "state"
)

// Level is a coarse severity for filtering and display.
type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Entry is one observation stored in the session hub.
type Entry struct {
	ID      int64             `json:"id"`
	Kind    Kind              `json:"kind"`
	Level   Level             `json:"level"`
	At      time.Time         `json:"at"`
	Source  string            `json:"source,omitempty"`
	Message string            `json:"message"`
	Meta    map[string]string `json:"meta,omitempty"`
	Turn    int               `json:"turn,omitempty"`
}

// Stats summarizes hub contents for status tools.
type Stats struct {
	TotalEntries   int            `json:"totalEntries"`
	ByKind         map[Kind]int   `json:"byKind"`
	ErrorCount     int            `json:"errorCount"`
	LastActivityAt time.Time      `json:"lastActivityAt,omitempty"`
	StateKeys      int            `json:"stateKeys"`
}

// Limits configures ring-buffer retention.
type Limits struct {
	MaxEntries int
}

// DefaultLimits returns production defaults for one session.
func DefaultLimits() Limits {
	return Limits{MaxEntries: 4096}
}

// ResolvedLimits normalizes user config into Limits.
func ResolvedLimits(maxEntries int) Limits {
	if maxEntries <= 0 {
		return DefaultLimits()
	}
	if maxEntries > 65536 {
		maxEntries = 65536
	}
	return Limits{MaxEntries: maxEntries}
}
