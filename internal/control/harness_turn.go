package control

import (
	"context"
	"errors"

	"arcdesk/internal/agent"
	"arcdesk/internal/harness"
)

func (c *Controller) beginHarnessTurnWithCtx(ctx context.Context, raw string) (planMode bool, autoEntered bool) {
	c.mu.Lock()
	wasPlan := c.planMode
	c.mu.Unlock()
	c.maybeAutoPlan(ctx, raw)
	c.mu.Lock()
	planMode = c.planMode
	autoEntered = !wasPlan && planMode
	c.mu.Unlock()
	return planMode, autoEntered
}

func (c *Controller) emitHarnessBegin(planMode, autoEntered bool) {
	if c.plane == nil {
		return
	}
	c.plane.FSM.BeginTurn(planMode, autoEntered)
	if planMode {
		c.plane.EmitPhase(harness.PhasePlan)
	} else {
		c.plane.EmitPhase(harness.PhaseExecute)
	}
}

func (c *Controller) harnessAfterRun(err error) {
	if c.plane == nil || err == nil {
		return
	}
	if errors.Is(err, agent.ErrVerifyExhausted) {
		c.plane.ApplyFSM(harness.EventVerifyExhaust)
	}
}

func (c *Controller) harnessTurnComplete() {
	if c.plane == nil {
		return
	}
	c.plane.ApplyFSM(harness.EventTurnComplete)
}

func (c *Controller) harnessPlanRejected() {
	if c.plane == nil {
		return
	}
	c.plane.ApplyFSM(harness.EventPlanRejected)
	c.plane.ApplyFSM(harness.EventTurnComplete)
}

func (c *Controller) harnessPlanApproved() {
	if c.plane == nil {
		return
	}
	c.plane.ApplyFSM(harness.EventPlanApproved)
	c.plane.EmitPhase(harness.PhaseExecute)
}

func (c *Controller) harnessEpisodicBound() {
	if c.plane == nil || c.plane.Memory == nil {
		return
	}
	_, _ = c.plane.Memory.Consolidate(harness.ConsolidateEpisodicBound)
}

func (c *Controller) queueWorkingMemory(note string) {
	if c.plane != nil && c.plane.Memory != nil {
		c.plane.Memory.QueueWorking(note)
	}
}

// Plane exposes the session Harness for tools and tests.
func (c *Controller) Plane() *harness.Plane {
	return c.plane
}
