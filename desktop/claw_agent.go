package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"arcdesk/internal/boot"
	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/provider"
)

func clawLastAssistantText(msgs []provider.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == provider.RoleAssistant && strings.TrimSpace(msgs[i].Content) != "" {
			return msgs[i].Content
		}
	}
	return ""
}

// clawChannelFromActiveTab builds mobile-agent settings from the desktop's active tab.
func (a *App) clawChannelFromActiveTab() ClawChannel {
	ch := ClawChannel{Model: "deepseek-chat"}
	if a == nil {
		return ch
	}
	a.mu.RLock()
	tab := a.activeTabLocked()
	a.mu.RUnlock()
	if tab != nil {
		if m := strings.TrimSpace(tab.model); m != "" {
			ch.Model = m
		}
		ch.WorkspaceRoot = strings.TrimSpace(tab.WorkspaceRoot)
	}
	root := strings.TrimSpace(ch.WorkspaceRoot)
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
			ch.WorkspaceRoot = root
		}
	}
	if cfg, err := config.LoadForRoot(root); err == nil {
		if sp := strings.TrimSpace(cfg.Agent.SystemPrompt); sp != "" {
			ch.Persona = sp
		}
	}
	return ch
}

func (a *App) runClawAgentReply(ch ClawChannel, userText string) (string, error) {
	text := strings.TrimSpace(userText)
	if text == "" {
		return "", fmt.Errorf("message text is required")
	}
	ctx, cancel := context.WithTimeout(a.bootContext(), 8*time.Minute)
	defer cancel()

	root := strings.TrimSpace(ch.WorkspaceRoot)
	if root == "" {
		root = "."
	}
	model := strings.TrimSpace(ch.Model)
	if model == "" {
		model = "deepseek-chat"
	}

	ctrl, err := boot.Build(ctx, boot.Options{
		Model:         model,
		Sink:          event.Discard,
		WorkspaceRoot: root,
		RequireKey:    false,
	})
	if err != nil {
		return "", err
	}
	defer ctrl.Close()

	input := text
	if persona := strings.TrimSpace(ch.Persona); persona != "" {
		input = persona + "\n\n" + text
	}
	if err := ctrl.Run(ctx, input); err != nil {
		return "", err
	}
	reply := strings.TrimSpace(clawLastAssistantText(ctrl.History()))
	if reply == "" {
		return "（暂无文本回复）", nil
	}
	return reply, nil
}
