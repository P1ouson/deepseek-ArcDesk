package selfdebug

import (
	"sync"
)

// Tracker holds live self-debug loop state for tools and diagnostics.
type Tracker struct {
	mu   sync.RWMutex
	plan Plan
	snap Snapshot
}

// NewTracker returns a tracker for the resolved verification plan.
func NewTracker(plan Plan) *Tracker {
	return &Tracker{plan: plan}
}

// Update records the latest loop snapshot.
func (t *Tracker) Update(snap Snapshot) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.snap = snap
	t.mu.Unlock()
}

// Snapshot returns the latest loop state.
func (t *Tracker) Snapshot() Snapshot {
	if t == nil {
		return Snapshot{}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.snap
}

// Plan returns the configured verification plan.
func (t *Tracker) Plan() Plan {
	if t == nil {
		return Plan{}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.plan
}
