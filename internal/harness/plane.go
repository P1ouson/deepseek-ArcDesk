package harness

import (
	"sync"

	"arcdesk/internal/event"
)

// Plane is the unified Harness control plane for one session.
type Plane struct {
	mu sync.RWMutex

	FSM    *FSM
	Memory *FourLayer
	Layers LayerFlags
	live   LiveLayers

	sink event.Sink
}

// Config wires a session-scoped plane.
type Config struct {
	Layers LayerFlags
	Sink   event.Sink
}

// NewPlane builds the control plane. Sink may be nil (events discarded).
func NewPlane(cfg Config) *Plane {
	sink := cfg.Sink
	if sink == nil {
		sink = event.Discard
	}
	mem := NewFourLayer()
	mem.SetSemanticLoaded(cfg.Layers.Memory)
	return &Plane{
		FSM:    NewFSM(),
		Memory: mem,
		Layers: cfg.Layers,
		sink:   sink,
	}
}

// BindSession updates episodic memory path when the session file rotates.
func (p *Plane) BindSession(sessionPath string) {
	if p == nil || p.Memory == nil {
		return
	}
	p.Memory.BindSession(sessionPath)
}

// OnSemanticRefresh registers semantic consolidate callback.
func (p *Plane) OnSemanticRefresh(fn func()) {
	if p == nil || p.Memory == nil {
		return
	}
	p.Memory.OnSemanticRefresh(fn)
}

// EmitPhase is internal-only; harness FSM state is exposed via harness_status,
// not the chat timeline (event.Phase is reserved for coordinator handoffs).
func (p *Plane) EmitPhase(TurnPhase) {}

// ApplyFSM transitions the FSM without emitting chat timeline noise.
func (p *Plane) ApplyFSM(ev TurnEvent) TurnPhase {
	if p == nil || p.FSM == nil {
		return PhaseIdle
	}
	return p.FSM.Apply(ev)
}

// Status is returned by harness_status and debugging APIs.
type Status struct {
	Phase         TurnPhase    `json:"phase"`
	VerifyRetries int          `json:"verify_retries"`
	Layers        []LayerEntry `json:"layers"`
	Memory        map[string]interface{} `json:"memory"`
}

// Status returns a snapshot of plane + layers + memory.
func (p *Plane) Status() Status {
	if p == nil {
		return Status{}
	}
	st := Status{
		Layers: DescribeLayers(p.effectiveLayerFlags()),
		Memory: p.effectiveMemorySnapshot(),
	}
	if p.FSM != nil {
		st.Phase = p.FSM.Phase()
		st.VerifyRetries = p.FSM.VerifyRetries()
	}
	return st
}

// SetMCPServerCount updates MCP layer count (hot-add/remove).
func (p *Plane) SetMCPServerCount(n int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Layers.MCP = n
}
