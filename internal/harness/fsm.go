package harness

import (
	"fmt"
	"sync"
)

// TurnPhase is the controller-visible stage of PLAN-EXECUTE-VERIFY.
// VERIFY runs inside the agent execute loop; the FSM exposes it when the
// executor reports verification retries or exhaustion.
type TurnPhase string

const (
	PhaseIdle    TurnPhase = "idle"
	PhasePlan    TurnPhase = "plan"
	PhaseExecute TurnPhase = "execute"
	PhaseVerify  TurnPhase = "verify"
	PhaseReplan  TurnPhase = "replan"
	PhaseHalt    TurnPhase = "halt"
)

// TurnEvent drives deterministic FSM transitions.
type TurnEvent string

const (
	EventTurnStart      TurnEvent = "turn_start"
	EventPlanResearch   TurnEvent = "plan_research"
	EventPlanRejected   TurnEvent = "plan_rejected"
	EventPlanApproved   TurnEvent = "plan_approved"
	EventExecuteStart   TurnEvent = "execute_start"
	EventExecuteDone    TurnEvent = "execute_done"
	EventVerifyActive   TurnEvent = "verify_active"
	EventVerifyExhaust  TurnEvent = "verify_exhausted"
	EventTurnComplete   TurnEvent = "turn_complete"
	EventHalt           TurnEvent = "halt"
)

// FSM holds the current turn phase for observability and gate decisions.
type FSM struct {
	mu    sync.RWMutex
	phase TurnPhase
	// verifyRetries counts controller-visible verify blocks this execute phase.
	verifyRetries int
}

// NewFSM returns an FSM in idle.
func NewFSM() *FSM {
	return &FSM{phase: PhaseIdle}
}

// Phase returns the current phase.
func (f *FSM) Phase() TurnPhase {
	if f == nil {
		return PhaseIdle
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.phase
}

// VerifyRetries returns how many verify blocks occurred this execute phase.
func (f *FSM) VerifyRetries() int {
	if f == nil {
		return 0
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.verifyRetries
}

// BeginTurn selects the entry phase for a new user turn.
func (f *FSM) BeginTurn(planMode bool, autoPlan bool) TurnPhase {
	if f == nil {
		return PhaseExecute
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.verifyRetries = 0
	switch {
	case planMode || autoPlan:
		f.phase = PhasePlan
	default:
		f.phase = PhaseExecute
	}
	return f.phase
}

// Apply applies an event and returns the new phase. Invalid transitions are
// ignored and the current phase is returned unchanged.
func (f *FSM) Apply(ev TurnEvent) TurnPhase {
	if f == nil {
		return PhaseIdle
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	next, ok := transition(f.phase, ev)
	if !ok {
		return f.phase
	}
	if ev == EventVerifyActive {
		f.verifyRetries++
	}
	if ev == EventTurnComplete || ev == EventHalt {
		f.verifyRetries = 0
	}
	f.phase = next
	return f.phase
}

func transition(from TurnPhase, ev TurnEvent) (TurnPhase, bool) {
	switch from {
	case PhaseIdle:
		switch ev {
		case EventTurnStart, EventExecuteStart:
			return PhaseExecute, true
		case EventPlanResearch:
			return PhasePlan, true
		case EventHalt:
			return PhaseHalt, true
		}
	case PhasePlan:
		switch ev {
		case EventPlanRejected:
			return PhaseReplan, true
		case EventPlanApproved, EventExecuteStart:
			return PhaseExecute, true
		case EventHalt:
			return PhaseHalt, true
		}
	case PhaseReplan:
		switch ev {
		case EventPlanResearch:
			return PhasePlan, true
		case EventTurnComplete:
			return PhaseIdle, true
		case EventHalt:
			return PhaseHalt, true
		}
	case PhaseExecute:
		switch ev {
		case EventVerifyActive:
			return PhaseVerify, true
		case EventExecuteDone, EventTurnComplete:
			return PhaseIdle, true
		case EventVerifyExhaust:
			return PhaseHalt, true
		case EventHalt:
			return PhaseHalt, true
		}
	case PhaseVerify:
		switch ev {
		case EventExecuteDone, EventTurnComplete:
			return PhaseIdle, true
		case EventVerifyActive:
			return PhaseVerify, true
		case EventVerifyExhaust:
			return PhaseHalt, true
		case EventExecuteStart:
			return PhaseExecute, true
		case EventHalt:
			return PhaseHalt, true
		}
	case PhaseHalt:
		switch ev {
		case EventTurnComplete:
			return PhaseIdle, true
		}
	}
	return from, false
}

// PhaseLabel formats a phase for event.Phase text.
func PhaseLabel(p TurnPhase) string {
	return "harness/" + string(p)
}

// ErrHalted is returned when the FSM is in halt after verify exhaustion.
var ErrHalted = fmt.Errorf("harness: turn halted")
