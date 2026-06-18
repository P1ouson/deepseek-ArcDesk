package main

import (
	"errors"
	"testing"

	"arcdesk/internal/i18n"
)

func TestUserFacingErr(t *testing.T) {
	got := userFacingMsg(errors.New(`deepseek-flash: request failed: Post "https://zenmux.ai/api/v1/chat/completions": context canceled`))
	if got != i18n.M.AgentRequestCanceled {
		t.Fatalf("userFacingMsg = %q, want %q", got, i18n.M.AgentRequestCanceled)
	}
}
