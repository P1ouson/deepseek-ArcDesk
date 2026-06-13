package planner

import (
	"fmt"
	"strings"
	"sync"
)

// Status is a snapshot of phased execution progress.
type Status struct {
	Active         bool     `json:"active"`
	TotalPhases    int      `json:"total_phases"`
	CurrentIndex   int      `json:"current_index"` // 0-based; equals TotalPhases when finished
	CurrentTitle   string   `json:"current_title,omitempty"`
	CompletedCount int      `json:"completed_count"`
	Summaries      []string `json:"summaries,omitempty"`
	EnforceGates   bool     `json:"enforce_gates"`
}

// Tracker holds live phased-plan state for tools and final-answer gating.
type Tracker struct {
	mu           sync.RWMutex
	phases       []Phase
	current      int
	summaries    []string
	enforceGates bool
	armed        bool // true after LoadFromPlan until Clear
}

// NewTracker returns a tracker; enforceGates blocks final answers until all
// phases are explicitly completed via planner_complete_phase.
func NewTracker(enforceGates bool) *Tracker {
	return &Tracker{current: -1, enforceGates: enforceGates}
}

// LoadFromPlan parses markdown and starts phase 0 when phases exist.
func (t *Tracker) LoadFromPlan(plan string) {
	if t == nil {
		return
	}
	phases := ParsePhases(plan)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.phases = phases
	t.summaries = nil
	if len(phases) == 0 {
		t.current = -1
		t.armed = false
		return
	}
	t.current = 0
	t.armed = true
}

// Clear drops phased state (e.g. when a turn skips planning for a low-risk ask).
func (t *Tracker) Clear() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.phases = nil
	t.summaries = nil
	t.current = -1
	t.armed = false
}

// LoadPhases replaces the active plan programmatically.
func (t *Tracker) LoadPhases(phases []Phase) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.phases = append([]Phase(nil), phases...)
	t.summaries = nil
	if len(t.phases) == 0 {
		t.current = -1
		t.armed = false
		return
	}
	t.current = 0
	t.armed = true
}

// Status returns the current phased execution snapshot.
func (t *Tracker) Status() Status {
	if t == nil {
		return Status{}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	st := Status{
		TotalPhases:    len(t.phases),
		CurrentIndex:   t.current,
		CompletedCount: len(t.summaries),
		Summaries:      append([]string(nil), t.summaries...),
		EnforceGates:   t.enforceGates,
	}
	st.Active = len(t.phases) > 0 && t.current >= 0 && t.current < len(t.phases)
	if st.Active {
		st.CurrentTitle = t.phases[t.current].Title
	}
	return st
}

// CompleteCurrent marks the active phase done and advances to the next.
func (t *Tracker) CompleteCurrent(summary string) (nextFocus string, err error) {
	if t == nil {
		return "", fmt.Errorf("planner not configured")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.phases) == 0 || t.current < 0 || t.current >= len(t.phases) {
		return "", fmt.Errorf("no active phase to complete")
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = t.phases[t.current].Title
	}
	t.summaries = append(t.summaries, summary)
	t.current++
	if t.current >= len(t.phases) {
		return "All phases complete. You may finish the turn.", nil
	}
	return t.focusLocked(t.current), nil
}

// BlockFinalAnswer returns a readiness hint when gates are enforced and phases
// remain incomplete.
func (t *Tracker) BlockFinalAnswer() string {
	if t == nil || !t.enforceGates {
		return ""
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.armed || len(t.phases) == 0 || t.current < 0 {
		return ""
	}
	if t.current >= len(t.phases) {
		return ""
	}
	phase := t.phases[t.current]
	return fmt.Sprintf("phased plan: complete phase %d/%d (%q) via planner_complete_phase before finishing — do not implement later phases in one shot",
		t.current+1, len(t.phases), phase.Title)
}

// CurrentFocusBlock returns instructions for the active phase only.
func (t *Tracker) CurrentFocusBlock() string {
	if t == nil {
		return ""
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.current < 0 || t.current >= len(t.phases) {
		return ""
	}
	return t.focusLocked(t.current)
}

func (t *Tracker) focusLocked(idx int) string {
	if idx < 0 || idx >= len(t.phases) {
		return ""
	}
	p := t.phases[idx]
	var b strings.Builder
	fmt.Fprintf(&b, "PHASED EXECUTION — work on phase %d/%d only:\n%s", idx+1, len(t.phases), p.Title)
	if len(p.Steps) > 0 {
		b.WriteString("\nSub-steps:")
		for _, s := range p.Steps {
			b.WriteString("\n- ")
			b.WriteString(s)
		}
	}
	b.WriteString("\nFinish this phase, verify, then call planner_complete_phase before starting the next phase.")
	return b.String()
}
