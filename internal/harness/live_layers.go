package harness

// LiveLayers supplies runtime values that supersede boot-time LayerFlags snapshots
// when Status() is queried. Desktop defers MCP handshakes and skills/memory can
// change mid-session, so harness_status must read live state—not the boot count.
type LiveLayers struct {
	MCPServerCount func() int
	SkillCount     func() int
	SemanticLoaded func() bool
}

// BindLiveLayers wires session-scoped resolvers. Pass zero values to leave a field
// on the boot snapshot.
func (p *Plane) BindLiveLayers(l LiveLayers) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.live = l
}

func (p *Plane) effectiveLayerFlags() LayerFlags {
	p.mu.RLock()
	flags := p.Layers
	live := p.live
	p.mu.RUnlock()
	if live.MCPServerCount != nil {
		flags.MCP = live.MCPServerCount()
	}
	if live.SkillCount != nil {
		flags.Skills = live.SkillCount()
	}
	return flags
}

func (p *Plane) effectiveMemorySnapshot() map[string]interface{} {
	snap := p.Memory.Snapshot()
	if snap == nil {
		return snap
	}
	p.mu.RLock()
	fn := p.live.SemanticLoaded
	p.mu.RUnlock()
	if fn != nil {
		snap["semantic_loaded"] = fn()
	}
	return snap
}
