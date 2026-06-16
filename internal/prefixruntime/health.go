package prefixruntime

import (
	"fmt"

	"arcdesk/internal/provider"
	"arcdesk/internal/usage"
)

// HealthAction describes runtime self-healing when session cache hit rate drops.
type HealthAction struct {
	LowHitRate       bool
	FreezeCompaction bool
	Notice           string
}

// HealthMonitor tracks session KV cache health and recommends heal actions.
type HealthMonitor struct {
	settings Settings
	steps    int
	freeze   bool
	alerted  bool
}

// NewHealthMonitor constructs a monitor; nil settings uses defaults with Enabled=true.
func NewHealthMonitor(settings Settings) *HealthMonitor {
	settings = settings.withDefaults()
	if !settings.Enabled {
		return nil
	}
	return &HealthMonitor{settings: settings}
}

// RecordStep ingests one API usage row and returns recommended actions.
func (h *HealthMonitor) RecordStep(usageRow *provider.Usage, sessionHit, sessionMiss int) HealthAction {
	if h == nil {
		return HealthAction{}
	}
	h.steps++
	if usageRow == nil || usageRow.TotalTokens == 0 {
		return HealthAction{}
	}
	pct, ok := usage.SessionRates(sessionHit, sessionMiss)
	if !ok || h.steps < h.settings.MinStepsForAlert {
		return HealthAction{}
	}
	if pct/100 >= h.settings.MinHitRate {
		h.freeze = false
		h.alerted = false
		return HealthAction{}
	}
	h.freeze = true
	act := HealthAction{
		LowHitRate:       true,
		FreezeCompaction: true,
		Notice: fmt.Sprintf(
			"prefix cache hit rate %.0f%% below %.0f%% — freezing auto-compaction and deferring prefix rewrites until hit rate recovers",
			pct, h.settings.MinHitRate*100,
		),
	}
	if !h.alerted {
		h.alerted = true
		return act
	}
	act.Notice = ""
	return act
}

// FreezeCompaction reports whether auto-compaction should be deferred.
func (h *HealthMonitor) FreezeCompaction() bool {
	if h == nil {
		return false
	}
	return h.freeze
}

// StepHitRate returns the latest step hit rate from usage (for metrics).
func StepHitRate(u *provider.Usage) (pct float64, ok bool) {
	_, _, pct, ok = usage.StepRates(u)
	return pct, ok
}
