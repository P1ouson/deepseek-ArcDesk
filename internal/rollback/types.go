package rollback

import "arcdesk/internal/checkpoint"

// FileRevert is one file's structured undo diff (current → checkpoint state).
type FileRevert struct {
	Path    string `json:"path"`
	Action  string `json:"action"` // modify | create | delete
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Diff    string `json:"diff,omitempty"`
}

// Report is the structured auto-diff for one rewind turn.
type Report struct {
	Turn    int          `json:"turn"`
	Prompt  string       `json:"prompt,omitempty"`
	Files   []FileRevert `json:"files"`
	Summary string       `json:"summary"`
}

// Host reads checkpoint plans and workspace files for tools and retry context.
type Host struct {
	store func() *checkpoint.Store
	root  string
	turn  func() int
}

// NewHost wires checkpoint access. store and turn may be nil (no-op host).
func NewHost(store func() *checkpoint.Store, root string, turn func() int) *Host {
	return &Host{store: store, root: root, turn: turn}
}

// ActiveTurn returns the current user turn for rollback preview.
func (h *Host) ActiveTurn() int {
	if h == nil || h.turn == nil {
		return -1
	}
	return h.turn()
}

// Checkpoints lists checkpoint metadata when a store is available.
func (h *Host) Checkpoints() []checkpoint.Meta {
	if h == nil || h.store == nil {
		return nil
	}
	st := h.store()
	if st == nil {
		return nil
	}
	return st.List()
}

// Report builds the structured diff for rewinding from turn.
func (h *Host) Report(turn int) Report {
	if h == nil || h.store == nil || turn < 0 {
		return Report{Turn: turn}
	}
	st := h.store()
	if st == nil {
		return Report{Turn: turn}
	}
	return BuildReport(h.root, st.RestorePlan(turn))
}
