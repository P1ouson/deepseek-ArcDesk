package usage

import (
	"testing"

	"arcdesk/internal/provider"
)

func TestStepRatesMatchesReasonixDenominator(t *testing.T) {
	u := &provider.Usage{PromptTokens: 1000, CacheHitTokens: 900, CacheMissTokens: 100}
	_, _, pct, ok := StepRates(u)
	if !ok || pct < 89.99 || pct > 90.01 {
		t.Fatalf("step rate = %v ok=%v, want ~90%%", pct, ok)
	}
	u2 := &provider.Usage{PromptTokens: 1000, CacheHitTokens: 800, CacheMissTokens: 0}
	_, denom2, pct2, ok2 := StepRates(u2)
	if !ok2 || denom2 != 800 || pct2 < 99.99 {
		t.Fatalf("hit+miss denom when miss=0: denom=%d pct=%v", denom2, pct2)
	}
}

func TestSessionRates(t *testing.T) {
	pct, ok := SessionRates(800, 200)
	if !ok || pct < 79.99 || pct > 80.01 {
		t.Fatalf("session rate = %v", pct)
	}
}
