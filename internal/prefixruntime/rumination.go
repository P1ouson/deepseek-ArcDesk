package prefixruntime

import (
	"strings"
	"unicode/utf8"

	"arcdesk/internal/provider"
)

// RuminationSettings bounds thinking-only steps before forcing action.
type RuminationSettings struct {
	Enabled              bool
	MaxReasonOnlySteps   int
	MinReasoningRunes    int
	InterruptHint        string
}

func (s RuminationSettings) withDefaults() RuminationSettings {
	if s.MaxReasonOnlySteps <= 0 {
		s.MaxReasonOnlySteps = 2
	}
	if s.MinReasoningRunes <= 0 {
		s.MinReasoningRunes = 400
	}
	if strings.TrimSpace(s.InterruptHint) == "" {
		s.InterruptHint = "You have enough context. Take one concrete action now (tool call, patch, or verify command). Prefer execution over further speculation."
	}
	return s
}

// RuminationScheduler detects consecutive reasoning-only model steps.
type RuminationScheduler struct {
	settings RuminationSettings
	streak   int
}

// NewRuminationScheduler returns a scheduler; nil when disabled.
func NewRuminationScheduler(settings RuminationSettings) *RuminationScheduler {
	settings = settings.withDefaults()
	if !settings.Enabled {
		return nil
	}
	return &RuminationScheduler{settings: settings}
}

// ObserveStep updates streak from one agent step. When the streak exceeds the
// limit, returns a user-message hint to inject before the next completion.
func (r *RuminationScheduler) ObserveStep(calls []provider.ToolCall, reasoning, text string) string {
	if r == nil {
		return ""
	}
	if len(calls) > 0 || strings.TrimSpace(text) != "" {
		r.streak = 0
		return ""
	}
	if utf8.RuneCountInString(strings.TrimSpace(reasoning)) < r.settings.MinReasoningRunes {
		return ""
	}
	r.streak++
	if r.streak < r.settings.MaxReasonOnlySteps {
		return ""
	}
	r.streak = 0
	return r.settings.InterruptHint
}
