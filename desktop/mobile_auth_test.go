package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testMobileAuthBridge(t *testing.T) (*clawBridge, string, string) {
	t.Helper()
	isolateDesktopUserDirs(t)

	store := newMobileConnectStore()
	store.mu.Lock()
	store.config.Enabled = true
	pairToken := store.pairToken.Token
	store.mu.Unlock()

	sess, err := store.pair(pairToken)
	if err != nil {
		t.Fatalf("pair: %v", err)
	}

	app := &App{mobileDecision: newMobileDecisionStore()}
	bridge := &clawBridge{mobile: store, app: app, port: defaultClawBridgePort}
	return bridge, sess.ID, pairToken
}

func mobileSessionCount(store *mobileConnectStore) int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.sessions)
}

func postMobileSend(bridge *clawBridge, sessionID, text string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"sessionId": sessionID, "text": text})
	req := httptest.NewRequest(http.MethodPost, "/mobile/api/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	bridge.handleMobileSend(rec, req)
	return rec
}

func getMobileDecision(bridge *clawBridge, sessionID string) *httptest.ResponseRecorder {
	path := "/mobile/api/decision"
	if sessionID != "" {
		path += "?session=" + sessionID
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	bridge.handleMobileDecision(rec, req)
	return rec
}

func postMobileDecision(bridge *clawBridge, sessionID, decisionID string, allow bool) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]any{
		"sessionId":  sessionID,
		"decisionId": decisionID,
		"allow":      allow,
	})
	req := httptest.NewRequest(http.MethodPost, "/mobile/api/decision", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	bridge.handleMobileDecision(rec, req)
	return rec
}

func postMobilePair(bridge *clawBridge, token string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/mobile/api/pair", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	bridge.handleMobilePair(rec, req)
	return rec
}

func TestMobilePairCreatesSession(t *testing.T) {
	isolateDesktopUserDirs(t)
	store := newMobileConnectStore()
	store.mu.Lock()
	store.config.Enabled = true
	token := store.pairToken.Token
	store.mu.Unlock()

	bridge := &clawBridge{mobile: store, app: &App{}}
	before := mobileSessionCount(store)

	rec := postMobilePair(bridge, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("pair status = %d body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		SessionID string `json:"sessionId"`
		OK        bool   `json:"ok"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.SessionID == "" {
		t.Fatalf("pair response = %+v", resp)
	}
	if mobileSessionCount(store) != before+1 {
		t.Fatalf("expected one new session, got count %d (before %d)", mobileSessionCount(store), before)
	}
	if _, ok := store.session(resp.SessionID); !ok {
		t.Fatal("paired session not found in store")
	}
}

func TestMobileSendRejectsUnknownSession(t *testing.T) {
	bridge, _, _ := testMobileAuthBridge(t)
	before := mobileSessionCount(bridge.mobile)

	rec := postMobileSend(bridge, "forged-session-id", "hello")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 body=%s", rec.Code, rec.Body.String())
	}
	if mobileSessionCount(bridge.mobile) != before {
		t.Fatal("unknown send must not create sessions")
	}
}

func TestMobileSendRejectsForgedSession(t *testing.T) {
	bridge, pairedID, _ := testMobileAuthBridge(t)
	forged := pairedID + "-tampered"
	before := mobileSessionCount(bridge.mobile)

	rec := postMobileSend(bridge, forged, "hello")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if mobileSessionCount(bridge.mobile) != before {
		t.Fatal("forged send must not create sessions")
	}
}

func TestMobileSendRejectsEmptySession(t *testing.T) {
	bridge, _, _ := testMobileAuthBridge(t)
	rec := postMobileSend(bridge, "", "hello")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "session is required") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestMobileSendPairedOK(t *testing.T) {
	bridge, sessionID, _ := testMobileAuthBridge(t)
	rec := postMobileSend(bridge, sessionID, "hello agent")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	msgs := bridge.mobile.messages(sessionID)
	if len(msgs) < 2 {
		t.Fatalf("expected user + pending messages, got %d", len(msgs))
	}
}

func TestMobileDecisionGETRequiresSession(t *testing.T) {
	bridge, _, _ := testMobileAuthBridge(t)

	rec := getMobileDecision(bridge, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty session GET status = %d, want 400", rec.Code)
	}

	rec = getMobileDecision(bridge, "unknown-session")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown session GET status = %d, want 401", rec.Code)
	}
}

func TestMobileDecisionPOSTRejectsEmptySession(t *testing.T) {
	bridge, _, _ := testMobileAuthBridge(t)
	bridge.app.mobileDecision.set(&MobilePendingDecision{Kind: "approval", ID: "dec-1", TabID: "test", Title: "t"})

	rec := postMobileDecision(bridge, "", "dec-1", true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
}

func TestMobileDecisionPOSTRejectsUnknownSession(t *testing.T) {
	bridge, _, _ := testMobileAuthBridge(t)
	bridge.app.mobileDecision.set(&MobilePendingDecision{Kind: "approval", ID: "dec-1", TabID: "test", Title: "t"})

	rec := postMobileDecision(bridge, "not-paired", "dec-1", true)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestMobileDecisionPairedOK(t *testing.T) {
	bridge, sessionID, _ := testMobileAuthBridge(t)
	bridge.app.mobileDecision.set(&MobilePendingDecision{
		Kind: "approval", ID: "dec-1", TabID: "test", Title: "approve bash", Tool: "bash",
	})

	getRec := getMobileDecision(bridge, sessionID)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d body=%s", getRec.Code, getRec.Body.String())
	}
	var getResp struct {
		Decision *MobilePendingDecision `json:"decision"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatal(err)
	}
	if getResp.Decision == nil || getResp.Decision.ID != "dec-1" {
		t.Fatalf("GET decision = %+v", getResp.Decision)
	}

	postRec := postMobileDecision(bridge, sessionID, "dec-1", true)
	if postRec.Code != http.StatusOK {
		t.Fatalf("POST status = %d body=%s", postRec.Code, postRec.Body.String())
	}
	if bridge.app.GetMobilePendingDecision() != nil {
		t.Fatal("decision should be cleared after POST")
	}
}

func TestMobileSendDoesNotCreateSessionsOnFailure(t *testing.T) {
	bridge, _, _ := testMobileAuthBridge(t)
	before := mobileSessionCount(bridge.mobile)

	for _, id := range []string{"evil-1", "evil-2", ""} {
		_ = postMobileSend(bridge, id, "probe")
	}
	if mobileSessionCount(bridge.mobile) != before {
		t.Fatalf("session count changed from %d to %d after failed sends", before, mobileSessionCount(bridge.mobile))
	}
}
