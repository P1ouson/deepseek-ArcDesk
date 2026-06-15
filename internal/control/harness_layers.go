package control

import "arcdesk/internal/harness"

func (c *Controller) bindHarnessLiveLayers() {
	if c.plane == nil {
		return
	}
	c.plane.BindLiveLayers(harness.LiveLayers{
		MCPServerCount: c.harnessMCPServerCount,
		SkillCount:     c.harnessSkillCount,
		SemanticLoaded: c.harnessSemanticLoaded,
	})
}

func (c *Controller) harnessMCPServerCount() int {
	if c.host == nil {
		return 0
	}
	return len(c.host.ServerNames())
}

func (c *Controller) harnessSkillCount() int {
	c.mu.Lock()
	n := len(c.skills)
	c.mu.Unlock()
	return n
}

func (c *Controller) harnessSemanticLoaded() bool {
	c.mu.Lock()
	mem := c.mem
	c.mu.Unlock()
	return harness.SemanticLoaded(mem)
}

func (c *Controller) syncHarnessMCP() {
	if c.plane == nil {
		return
	}
	c.plane.SetMCPServerCount(c.harnessMCPServerCount())
}

func (c *Controller) syncHarnessSemantic() {
	if c.plane == nil || c.plane.Memory == nil {
		return
	}
	c.plane.Memory.SetSemanticLoaded(c.harnessSemanticLoaded())
}

// syncHarnessSemanticLocked updates semantic tier status. Caller must hold c.mu.
func (c *Controller) syncHarnessSemanticLocked() {
	if c.plane == nil || c.plane.Memory == nil {
		return
	}
	c.plane.Memory.SetSemanticLoaded(harness.SemanticLoaded(c.mem))
}
