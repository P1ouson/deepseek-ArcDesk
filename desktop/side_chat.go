package main

import (
	"fmt"
	"strings"
)

// SideChatReply answers a lightweight side-thread question without touching the
// main agent loop or writer tools.
func (a *App) SideChatReply(userText string) (string, error) {
	text := strings.TrimSpace(userText)
	if text == "" {
		return "", fmt.Errorf("message is required")
	}
	a.mu.RLock()
	tabID := ""
	if tab := a.activeTabLocked(); tab != nil {
		tabID = tab.ID
	}
	a.mu.RUnlock()

	prompt := fmt.Sprintf(
		"Answer briefly in the same language as the user. Do not propose running commands or editing files unless explicitly asked.\n\nUser: %s",
		text,
	)
	reply, err := a.chatCompletionHTTP(
		tabID,
		"You are a concise side assistant in a coding IDE.",
		prompt,
		480,
		0.35,
	)
	if err != nil {
		return "", err
	}
	if reply == "" {
		return "（暂无回复）", nil
	}
	return reply, nil
}
