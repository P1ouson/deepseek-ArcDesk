package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// MobileAskOption is one choice in a mobile ask decision.
type MobileAskOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// MobileAskQuestion is one question surfaced to paired phones.
type MobileAskQuestion struct {
	ID      string            `json:"id"`
	Header  string            `json:"header,omitempty"`
	Prompt  string            `json:"prompt"`
	Options []MobileAskOption `json:"options"`
	Multi   bool              `json:"multi,omitempty"`
}

// MobilePendingDecision is surfaced to paired phones while the desktop waits.
type MobilePendingDecision struct {
	Kind      string              `json:"kind"` // approval | ask
	ID        string              `json:"id"`
	TabID     string              `json:"tabId"`
	Title     string              `json:"title"`
	Summary   string              `json:"summary"`
	Tool      string              `json:"tool,omitempty"`
	Questions []MobileAskQuestion `json:"questions,omitempty"`
}

type mobileDecisionStore struct {
	mu      sync.Mutex
	pending *MobilePendingDecision
}

func newMobileDecisionStore() *mobileDecisionStore {
	return &mobileDecisionStore{}
}

func (s *mobileDecisionStore) set(d *MobilePendingDecision) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d == nil || strings.TrimSpace(d.ID) == "" {
		s.pending = nil
		return
	}
	copy := *d
	s.pending = &copy
}

func (s *mobileDecisionStore) get() *MobilePendingDecision {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		return nil
	}
	copy := *s.pending
	return &copy
}

func (s *mobileDecisionStore) clear(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != nil && (id == "" || s.pending.ID == id) {
		s.pending = nil
	}
}

func (a *App) GetMobilePendingDecision() *MobilePendingDecision {
	if a == nil || a.mobileDecision == nil {
		return nil
	}
	return a.mobileDecision.get()
}

// RespondMobileDecision resolves a pending mobile decision. answers may carry ask selections.
func (a *App) RespondMobileDecision(decisionID string, allow bool, answers []QuestionAnswer) error {
	if a == nil || a.mobileDecision == nil {
		return fmt.Errorf("mobile decision unavailable")
	}
	pending := a.mobileDecision.get()
	if pending == nil || pending.ID != decisionID {
		return fmt.Errorf("no matching pending decision")
	}
	switch pending.Kind {
	case "approval":
		a.ApproveTab(pending.TabID, pending.ID, allow, false, false)
	case "ask":
		if len(answers) > 0 {
			a.AnswerQuestionForTab(pending.TabID, pending.ID, answers)
		} else if !allow {
			a.AnswerQuestionForTab(pending.TabID, pending.ID, nil)
		} else {
			return fmt.Errorf("structured ask requires option selection")
		}
	default:
		return fmt.Errorf("unknown decision kind")
	}
	a.mobileDecision.clear(decisionID)
	a.broadcastMobileDecision(nil)
	return nil
}

func (a *App) broadcastMobileDecision(d *MobilePendingDecision) {
	if a == nil || a.clawBridge == nil || a.clawBridge.mobile == nil {
		if a != nil && a.mobileDecision != nil {
			a.mobileDecision.set(d)
		}
		return
	}
	a.mobileDecision.set(d)
	for _, sess := range a.clawBridge.mobile.listSessions() {
		var msg mobileChatMessage
		if d == nil {
			msg = mobileChatMessage{
				ID:        "decision-clear",
				Text:      "",
				Outgoing:  true,
				Status:    "decision_clear",
				CreatedAt: time.Now().UnixMilli(),
			}
		} else {
			msg = mobileChatMessage{
				ID:        "decision-" + d.ID,
				Text:      mobileDecisionText(d),
				Outgoing:  true,
				Status:    "decision",
				CreatedAt: time.Now().UnixMilli(),
			}
		}
		a.clawBridge.emitMobileMessage(sess.ID, msg)
		a.clawBridge.pushRelayMessages(sess.ID)
	}
}

func mobileDecisionText(d *MobilePendingDecision) string {
	if d == nil {
		return ""
	}
	if d.Summary != "" {
		return d.Title + "\n" + d.Summary
	}
	return d.Title
}

func (b *clawBridge) handleMobileDecision(w http.ResponseWriter, r *http.Request) {
	if b == nil || b.app == nil {
		writeMobileJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "unavailable"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeMobileJSON(w, http.StatusOK, map[string]any{"decision": b.app.GetMobilePendingDecision()})
	case http.MethodPost:
		var req struct {
			SessionID  string           `json:"sessionId"`
			DecisionID string           `json:"decisionId"`
			Allow      bool             `json:"allow"`
			Answers    []QuestionAnswer `json:"answers"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeMobileJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
			return
		}
		if req.SessionID != "" {
			if _, ok := b.mobile.session(req.SessionID); !ok {
				writeMobileJSON(w, http.StatusUnauthorized, map[string]any{"error": "session not found"})
				return
			}
		}
		if err := b.app.RespondMobileDecision(req.DecisionID, req.Allow, req.Answers); err != nil {
			writeMobileJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeMobileJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
