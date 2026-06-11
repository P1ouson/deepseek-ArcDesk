// Package usage holds prompt-cache hit-rate helpers shared by the CLI, desktop
// wire, and cacheprobe. Formulas match Reasonix chat TUI (main-v2):
//   step  = hit / (hit+miss), falling back to hit / prompt
//   session = Σhit / (Σhit + Σmiss)
package usage

import "arcdesk/internal/provider"

// HitRate returns hit%, denominator, and ok. Matches Reasonix cacheRateLabel input.
func HitRate(hit, denom int) (pct float64, ok bool) {
	if denom <= 0 {
		return 0, false
	}
	return float64(hit) * 100 / float64(denom), true
}

// StepRates derives the latest API call cache hit rate from provider usage.
func StepRates(u *provider.Usage) (hit, denom int, pct float64, ok bool) {
	if u == nil {
		return 0, 0, 0, false
	}
	hit = u.CacheHitTokens
	denom = u.CacheHitTokens + u.CacheMissTokens
	if denom == 0 {
		denom = u.PromptTokens
	}
	pct, ok = HitRate(hit, denom)
	return hit, denom, pct, ok
}

// SessionRates derives the session-aggregate cache hit rate (Σhit/Σ(hit+miss)).
func SessionRates(sessionHit, sessionMiss int) (pct float64, ok bool) {
	return HitRate(sessionHit, sessionHit+sessionMiss)
}
