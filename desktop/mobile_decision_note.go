package main

import (
	"strings"

	"arcdesk/internal/event"
)

func (a *App) noteAgentDecision(tabID string, e event.Event) {
	if a == nil || a.mobileDecision == nil {
		return
	}
	switch e.Kind {
	case event.ApprovalRequest:
		subject := strings.TrimSpace(e.Approval.Subject)
		title := "需要批准：" + e.Approval.Tool
		if e.Approval.Tool == "exit_plan_mode" {
			title = "计划已就绪，等待你决定"
		}
		summary := subject
		if len(summary) > 220 {
			summary = summary[:217] + "…"
		}
		a.broadcastMobileDecision(&MobilePendingDecision{
			Kind:    "approval",
			ID:      e.Approval.ID,
			TabID:   tabID,
			Title:   title,
			Summary: summary,
			Tool:    e.Approval.Tool,
		})
		if a.decisionRoutes != nil {
			a.decisionRoutes.register(e.Approval.ID, tabID)
		}
	case event.AskRequest:
		q := ""
		if len(e.Ask.Questions) > 0 {
			q = strings.TrimSpace(e.Ask.Questions[0].Prompt)
		}
		if len(q) > 220 {
			q = q[:217] + "…"
		}
		questions := make([]MobileAskQuestion, 0, len(e.Ask.Questions))
		for _, question := range e.Ask.Questions {
			opts := make([]MobileAskOption, 0, len(question.Options))
			for _, opt := range question.Options {
				opts = append(opts, MobileAskOption{Label: opt.Label, Description: opt.Description})
			}
			questions = append(questions, MobileAskQuestion{
				ID:      question.ID,
				Header:  question.Header,
				Prompt:  question.Prompt,
				Options: opts,
				Multi:   question.Multi,
			})
		}
		a.broadcastMobileDecision(&MobilePendingDecision{
			Kind:      "ask",
			ID:        e.Ask.ID,
			TabID:     tabID,
			Title:     "需要你选择",
			Summary:   q,
			Questions: questions,
		})
		if a.decisionRoutes != nil {
			a.decisionRoutes.register(e.Ask.ID, tabID)
		}
	case event.TurnDone:
		a.broadcastMobileDecision(nil)
		if a.decisionRoutes != nil {
			a.decisionRoutes.clearAll()
		}
	}
}
