package harness

import (
	"fmt"
	"strings"
	"sync"
)

// MemoryLayer names the four memory tiers.
type MemoryLayer string

const (
	MemoryWorking    MemoryLayer = "working"
	MemoryEpisodic   MemoryLayer = "episodic"
	MemorySemantic   MemoryLayer = "semantic"
	MemoryProcedural MemoryLayer = "procedural"
)

// ConsolidateOp selects a cross-layer consolidation path.
type ConsolidateOp string

const (
	// ConsolidateWorkingToTurn drains working notes into the next turn compose
	// (automatic — called from Compose).
	ConsolidateWorkingToTurn ConsolidateOp = "working_to_turn"
	// ConsolidateSemanticRefresh reloads semantic docs into the cached prefix
	// snapshot held by the controller (mid-session prefix stays stable; index
	// metadata updates for status).
	ConsolidateSemanticRefresh ConsolidateOp = "semantic_refresh"
	// ConsolidateEpisodicBound records that the session transcript is the
	// episodic source of truth at this boundary (turn/checkpoint).
	ConsolidateEpisodicBound ConsolidateOp = "episodic_bound"
	// ConsolidateProceduralNote queues a procedural hint for verify-retry injection.
	ConsolidateProceduralNote ConsolidateOp = "procedural_note"
)

// ConsolidateResult summarizes what changed.
type ConsolidateResult struct {
	Op      ConsolidateOp `json:"op"`
	Layer   MemoryLayer   `json:"layer"`
	Count   int           `json:"count,omitempty"`
	Detail  string        `json:"detail,omitempty"`
}

// FourLayer implements Working / Episodic / Semantic / Procedural boundaries.
// Semantic content lives in memory.Set; episodic path is the session transcript;
// procedural notes are harness-managed strings destined for failure-memory hooks.
type FourLayer struct {
	mu sync.Mutex

	working []string
	// episodicPath is the session JSONL path (may be empty).
	episodicPath string
	// semanticLoaded reports whether hierarchical docs were loaded at boot.
	semanticLoaded bool
	// proceduralNotes are pending procedural consolidations (e.g. capture hints).
	proceduralNotes []string

	onSemanticRefresh func() // controller reloads mem index
}

// NewFourLayer returns an empty four-layer stack.
func NewFourLayer() *FourLayer {
	return &FourLayer{}
}

// BindSession wires the episodic transcript path.
func (m *FourLayer) BindSession(sessionPath string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.episodicPath = strings.TrimSpace(sessionPath)
}

// SetSemanticLoaded marks semantic tier availability.
func (m *FourLayer) SetSemanticLoaded(v bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.semanticLoaded = v
}

// OnSemanticRefresh registers a callback after semantic refresh consolidate.
func (m *FourLayer) OnSemanticRefresh(fn func()) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onSemanticRefresh = fn
}

// QueueWorking appends a working-memory note (turn-tail injection).
func (m *FourLayer) QueueWorking(note string) {
	note = strings.TrimSpace(note)
	if m == nil || note == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.working = append(m.working, note)
}

// DrainWorking returns and clears working notes for Compose.
func (m *FourLayer) DrainWorking() []string {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := append([]string(nil), m.working...)
	m.working = nil
	return out
}

// WorkingPending reports queued working notes without draining.
func (m *FourLayer) WorkingPending() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.working)
}

// QueueProcedural appends a procedural consolidation note.
func (m *FourLayer) QueueProcedural(note string) {
	note = strings.TrimSpace(note)
	if m == nil || note == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.proceduralNotes = append(m.proceduralNotes, note)
}

// Snapshot returns a JSON-friendly memory tier status.
func (m *FourLayer) Snapshot() map[string]interface{} {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]interface{}{
		"working_pending":    len(m.working),
		"episodic_path":      m.episodicPath,
		"semantic_loaded":    m.semanticLoaded,
		"procedural_pending": len(m.proceduralNotes),
	}
}

// Consolidate runs a cross-layer operation.
func (m *FourLayer) Consolidate(op ConsolidateOp) (ConsolidateResult, error) {
	if m == nil {
		return ConsolidateResult{}, fmt.Errorf("memory stack unavailable")
	}
	switch op {
	case ConsolidateWorkingToTurn:
		n := len(m.DrainWorking())
		return ConsolidateResult{Op: op, Layer: MemoryWorking, Count: n, Detail: "drained to turn compose"}, nil
	case ConsolidateSemanticRefresh:
		m.mu.Lock()
		fn := m.onSemanticRefresh
		m.mu.Unlock()
		if fn != nil {
			fn()
		}
		return ConsolidateResult{Op: op, Layer: MemorySemantic, Detail: "semantic index refreshed"}, nil
	case ConsolidateEpisodicBound:
		m.mu.Lock()
		path := m.episodicPath
		m.mu.Unlock()
		return ConsolidateResult{Op: op, Layer: MemoryEpisodic, Detail: path}, nil
	case ConsolidateProceduralNote:
		m.mu.Lock()
		n := len(m.proceduralNotes)
		m.proceduralNotes = nil
		m.mu.Unlock()
		return ConsolidateResult{Op: op, Layer: MemoryProcedural, Count: n}, nil
	default:
		return ConsolidateResult{}, fmt.Errorf("unknown consolidate op %q", op)
	}
}

// FormatWorkingBlock renders working notes for turn injection.
func FormatWorkingBlock(notes []string) string {
	if len(notes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<memory-update>\n")
	b.WriteString("The following project-memory changes were just made and apply from now on:\n")
	for _, n := range notes {
		b.WriteString("- " + n + "\n")
	}
	b.WriteString("</memory-update>\n\n")
	return b.String()
}
