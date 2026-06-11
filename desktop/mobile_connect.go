package main

import (
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	qrcode "github.com/skip2/go-qrcode"
)

//go:embed mobile_page.html
var mobilePageHTML []byte

const (
	mobilePairTokenTTL      = 15 * time.Minute
	mobileSessionIdleTTL    = 72 * time.Hour
	mobileSessionMaxAge     = 7 * 24 * time.Hour
	mobilePairTokenBytes    = 16
	mobileSessionBytes      = 24
	defaultTunnelIdleMin    = 10
	mobileSessionActiveWindow = 45 * time.Second
)

// errMobileSessionUnauthorized is returned when a mutating mobile API call
// references a session that was never created through the pairing flow.
var errMobileSessionUnauthorized = errors.New("session not found")

// MobileConnectConfig is persisted mobile-remote settings.
type MobileConnectConfig struct {
	Enabled                    bool   `json:"enabled"`
	AllowLAN                   bool   `json:"allowLAN"`
	Model                      string `json:"model"`
	Persona                    string `json:"persona"`
	WorkspaceRoot              string `json:"workspaceRoot"`
	RelayBaseURL               string `json:"relayBaseURL,omitempty"`
	DeviceID                   string `json:"deviceId,omitempty"`
	DeviceSecret               string `json:"deviceSecret,omitempty"`
	TunnelDisableIdleShutdown  bool   `json:"tunnelDisableIdleShutdown,omitempty"`
	TunnelIdleTimeoutMinutes   int    `json:"tunnelIdleTimeoutMinutes,omitempty"`
}

// MobilePairingInfo is shown on the desktop Connect page (QR + URL).
type MobilePairingInfo struct {
	Token          string `json:"token"`
	PairURL        string `json:"pairUrl"`
	LanPairURL     string `json:"lanPairUrl"`
	RelayURL       string `json:"relayUrl"`
	LanIP          string `json:"lanIp"`
	Port           int    `json:"port"`
	ExpiresAt      int64  `json:"expiresAt"`
	PairedCount    int    `json:"pairedCount"`
	ActiveCount    int    `json:"activeCount"`
	Enabled        bool   `json:"enabled"`
	QrDataURL      string `json:"qrDataUrl"`
	RelayConnected bool   `json:"relayConnected"`
	TunnelRunning  bool   `json:"tunnelRunning"`
	TunnelURL      string `json:"tunnelUrl"`
	ConnectMode    string `json:"connectMode"`
	BridgeReady    bool   `json:"bridgeReady"`
}

// MobileSessionSummary is a paired phone session for the desktop UI.
type MobileSessionSummary struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"createdAt"`
	LastSeen  int64  `json:"lastSeen"`
}

type mobileChatMessage struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Outgoing  bool   `json:"outgoing"`
	Status    string `json:"status,omitempty"`
	CreatedAt int64  `json:"createdAt"`
}

type mobilePairToken struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"`
}

type mobileSession struct {
	ID        string              `json:"id"`
	CreatedAt int64               `json:"createdAt"`
	LastSeen  int64               `json:"lastSeen"`
	Messages  []mobileChatMessage `json:"messages"`
}

type mobileConnectStore struct {
	mu          sync.Mutex
	pairToken   mobilePairToken
	config      MobileConnectConfig
	sessions    map[string]*mobileSession
	sessionPath string
}

func newMobileConnectStore() *mobileConnectStore {
	store := &mobileConnectStore{
		sessions:    map[string]*mobileSession{},
		sessionPath: ARCDESKDesktopDataPath("mobile-sessions.json"),
	}
	store.loadSessions()
	store.loadConfig()
	store.rotatePairToken()
	return store
}

func (s *mobileConnectStore) loadConfig() {
	path := ARCDESKDesktopDataPath("mobile-connect.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		s.config = MobileConnectConfig{Enabled: true, Model: "deepseek-chat", Persona: "Be concise and practical."}
		return
	}
	var cfg MobileConnectConfig
	if json.Unmarshal(raw, &cfg) == nil {
		s.config = cfg
	}
	s.config.AllowLAN = false
	if s.config.Model == "" {
		s.config.Model = "deepseek-chat"
	}
}

func (s *mobileConnectStore) saveConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveConfigLocked()
}

func (s *mobileConnectStore) saveConfigLocked() error {
	persist := s.config
	persist.AllowLAN = false
	return saveSensitiveJSON(ARCDESKDesktopDataPath("mobile-connect.json"), persist)
}

func (s *mobileConnectStore) loadSessions() {
	raw, err := os.ReadFile(s.sessionPath)
	if err != nil {
		return
	}
	var items []mobileSession
	if err := json.Unmarshal(raw, &items); err != nil {
		return
	}
	now := time.Now().UnixMilli()
	for i := range items {
		item := items[i]
		if !mobileSessionStillValid(&item, now) {
			continue
		}
		cp := item
		s.sessions[item.ID] = &cp
	}
}

func mobileSessionStillValid(sess *mobileSession, nowMs int64) bool {
	if sess == nil {
		return false
	}
	if nowMs-sess.LastSeen > mobileSessionIdleTTL.Milliseconds() {
		return false
	}
	if nowMs-sess.CreatedAt > mobileSessionMaxAge.Milliseconds() {
		return false
	}
	return true
}

func (s *mobileConnectStore) saveSessions() error {
	items := make([]mobileSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		items = append(items, *sess)
	}
	return saveSensitiveJSON(s.sessionPath, items)
}

func (s *mobileConnectStore) rotatePairToken() mobilePairToken {
	token := randomToken(mobilePairTokenBytes)
	pair := mobilePairToken{
		Token:     token,
		ExpiresAt: time.Now().Add(mobilePairTokenTTL).UnixMilli(),
	}
	s.pairToken = pair
	return pair
}

func (s *mobileConnectStore) getConfig() MobileConnectConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

func (s *mobileConnectStore) setConfig(cfg MobileConnectConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = "deepseek-chat"
	}
	s.config = cfg
	return s.saveConfigLocked()
}

func (s *mobileConnectStore) currentPairToken() (string, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Now().UnixMilli() > s.pairToken.ExpiresAt {
		s.rotatePairTokenLocked()
	}
	return s.pairToken.Token, s.pairToken.ExpiresAt
}

func mobileQRPairURL(primary, lan, relay string, allowLAN bool) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	if allowLAN && strings.TrimSpace(lan) != "" {
		return lan
	}
	return strings.TrimSpace(relay)
}

func buildMobilePairURLs(token string, port int, tunnelBase, relayBase string, relayOnline, allowLAN bool) (primary string, lan string, relay string, mode string) {
	if lanIP := primaryLANIP(); lanIP != "" {
		lan = fmt.Sprintf("http://%s:%d/mobile/p/%s", lanIP, port, token)
	}
	relay = ""
	if base := strings.TrimRight(strings.TrimSpace(relayBase), "/"); base != "" {
		relay = base + "/mobile/p/" + token
	}
	if base := strings.TrimRight(strings.TrimSpace(tunnelBase), "/"); base != "" {
		return base + "/mobile/p/" + token, lan, relay, "tunnel"
	}
	switch {
	case relayOnline && relay != "":
		return relay, lan, relay, "relay"
	case allowLAN && lan != "":
		return lan, lan, relay, "lan"
	case lan != "":
		return "", lan, relay, "lan_standby"
	default:
		return "", lan, relay, "none"
	}
}

func (s *mobileConnectStore) pairingInfo(port int, tunnelBase string, relayOnline bool) MobilePairingInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Now().UnixMilli() > s.pairToken.ExpiresAt {
		s.rotatePairTokenLocked()
	}
	tunnelBase = strings.TrimSpace(tunnelBase)
	primary, lan, relay, mode := buildMobilePairURLs(s.pairToken.Token, port, tunnelBase, s.config.RelayBaseURL, relayOnline, s.config.AllowLAN)
	now := time.Now().UnixMilli()
	return MobilePairingInfo{
		Token:          s.pairToken.Token,
		PairURL:        primary,
		LanPairURL:     lan,
		RelayURL:       relay,
		LanIP:          lanBindIP(),
		Port:           port,
		ExpiresAt:      s.pairToken.ExpiresAt,
		PairedCount:    len(s.sessions),
		ActiveCount:    s.activeSessionCountLocked(now),
		Enabled:        s.config.Enabled,
		QrDataURL:      mobileQRDataURL(mobileQRPairURL(primary, lan, relay, s.config.AllowLAN)),
		RelayConnected: relayOnline,
		TunnelRunning:  tunnelBase != "",
		TunnelURL:      tunnelBase,
		ConnectMode:    mode,
	}
}

func (s *mobileConnectStore) refreshPairing(port int, tunnelBase string, relayOnline bool) MobilePairingInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rotatePairTokenLocked()
	return s.pairingInfoLocked(port, tunnelBase, relayOnline)
}

func (s *mobileConnectStore) pairingInfoLocked(port int, tunnelBase string, relayOnline bool) MobilePairingInfo {
	tunnelBase = strings.TrimSpace(tunnelBase)
	primary, lan, relay, mode := buildMobilePairURLs(s.pairToken.Token, port, tunnelBase, s.config.RelayBaseURL, relayOnline, s.config.AllowLAN)
	now := time.Now().UnixMilli()
	return MobilePairingInfo{
		Token:          s.pairToken.Token,
		PairURL:        primary,
		LanPairURL:     lan,
		RelayURL:       relay,
		LanIP:          lanBindIP(),
		Port:           port,
		ExpiresAt:      s.pairToken.ExpiresAt,
		PairedCount:    len(s.sessions),
		ActiveCount:    s.activeSessionCountLocked(now),
		Enabled:        s.config.Enabled,
		QrDataURL:      mobileQRDataURL(mobileQRPairURL(primary, lan, relay, s.config.AllowLAN)),
		RelayConnected: relayOnline,
		TunnelRunning:  tunnelBase != "",
		TunnelURL:      tunnelBase,
		ConnectMode:    mode,
	}
}

func (s *mobileConnectStore) rotatePairTokenLocked() mobilePairToken {
	token := randomToken(mobilePairTokenBytes)
	s.pairToken = mobilePairToken{
		Token:     token,
		ExpiresAt: time.Now().Add(mobilePairTokenTTL).UnixMilli(),
	}
	return s.pairToken
}

func (s *mobileConnectStore) listSessions() []MobileSessionSummary {
	return s.listActiveSessions()
}

func (s *mobileConnectStore) listActiveSessions() []MobileSessionSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	window := mobileSessionActiveWindow.Milliseconds()
	out := make([]MobileSessionSummary, 0, len(s.sessions))
	for id, sess := range s.sessions {
		if !mobileSessionStillValid(sess, now) {
			delete(s.sessions, id)
			continue
		}
		if now-sess.LastSeen > window {
			continue
		}
		out = append(out, MobileSessionSummary{
			ID:        sess.ID,
			CreatedAt: sess.CreatedAt,
			LastSeen:  sess.LastSeen,
		})
	}
	return out
}

func (s *mobileConnectStore) listAllSessions() []MobileSessionSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	out := make([]MobileSessionSummary, 0, len(s.sessions))
	for id, sess := range s.sessions {
		if !mobileSessionStillValid(sess, now) {
			delete(s.sessions, id)
			continue
		}
		out = append(out, MobileSessionSummary{
			ID:        sess.ID,
			CreatedAt: sess.CreatedAt,
			LastSeen:  sess.LastSeen,
		})
	}
	return out
}

func (s *mobileConnectStore) pairedCount() int {
	return len(s.listSessions())
}

func (s *mobileConnectStore) activeSessionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeSessionCountLocked(time.Now().UnixMilli())
}

func (s *mobileConnectStore) activeSessionCountLocked(now int64) int {
	window := mobileSessionActiveWindow.Milliseconds()
	n := 0
	for id, sess := range s.sessions {
		if !mobileSessionStillValid(sess, now) {
			delete(s.sessions, id)
			continue
		}
		if now-sess.LastSeen <= window {
			n++
		}
	}
	return n
}

// requirePairedSession reports whether id names a session created by pair().
// It never creates sessions — unknown or expired ids return errMobileSessionUnauthorized.
func (s *mobileConnectStore) requirePairedSession(id string) error {
	key := strings.TrimSpace(id)
	if key == "" {
		return fmt.Errorf("session is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.config.Enabled {
		return fmt.Errorf("mobile connect is disabled")
	}
	sess, ok := s.sessions[key]
	if !ok {
		return errMobileSessionUnauthorized
	}
	now := time.Now().UnixMilli()
	if !mobileSessionStillValid(sess, now) {
		delete(s.sessions, key)
		_ = s.saveSessions()
		return errMobileSessionUnauthorized
	}
	sess.LastSeen = now
	return nil
}

func (s *mobileConnectStore) revokeSession(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strings.TrimSpace(id)
	if key == "" {
		return false
	}
	if _, ok := s.sessions[key]; !ok {
		return false
	}
	delete(s.sessions, key)
	_ = s.saveSessions()
	return true
}

func (s *mobileConnectStore) revokeAllSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sessions) == 0 {
		return
	}
	s.sessions = map[string]*mobileSession{}
	_ = s.saveSessions()
}

// clearActiveSessions marks every session offline immediately without deleting history.
func (s *mobileConnectStore) clearActiveSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	stale := now - mobileSessionActiveWindow.Milliseconds() - 1
	changed := false
	for _, sess := range s.sessions {
		if sess.LastSeen > stale {
			sess.LastSeen = stale
			changed = true
		}
	}
	if changed {
		_ = s.saveSessions()
	}
}

func (s *mobileConnectStore) pair(token string) (*mobileSession, error) {
	key := strings.TrimSpace(token)
	if key == "" {
		return nil, fmt.Errorf("pairing token is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.config.Enabled {
		return nil, fmt.Errorf("mobile connect is disabled")
	}
	if !pairTokenEqual(s.pairToken.Token, key) || time.Now().UnixMilli() > s.pairToken.ExpiresAt {
		return nil, fmt.Errorf("pairing token expired or invalid")
	}
	sess := &mobileSession{
		ID:        randomToken(mobileSessionBytes),
		CreatedAt: time.Now().UnixMilli(),
		LastSeen:  time.Now().UnixMilli(),
		Messages:  nil,
	}
	s.sessions[sess.ID] = sess
	s.rotatePairTokenLocked()
	_ = s.saveSessions()
	return sess, nil
}

func (s *mobileConnectStore) session(id string) (*mobileSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strings.TrimSpace(id)
	sess, ok := s.sessions[key]
	if !ok {
		return nil, false
	}
	now := time.Now().UnixMilli()
	if !mobileSessionStillValid(sess, now) {
		delete(s.sessions, key)
		_ = s.saveSessions()
		return nil, false
	}
	sess.LastSeen = now
	return sess, true
}

func (s *mobileConnectStore) appendMessage(sessionID, text string, outgoing bool, status string) (mobileChatMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return mobileChatMessage{}, fmt.Errorf("session not found")
	}
	msg := mobileChatMessage{
		ID:        fmt.Sprintf("m-%d", time.Now().UnixNano()),
		Text:      strings.TrimSpace(text),
		Outgoing:  outgoing,
		Status:    status,
		CreatedAt: time.Now().UnixMilli(),
	}
	sess.Messages = append(sess.Messages, msg)
	sess.LastSeen = time.Now().UnixMilli()
	_ = s.saveSessions()
	return msg, nil
}

func (s *mobileConnectStore) updateMessageStatus(sessionID, messageID, status, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return
	}
	for i := range sess.Messages {
		if sess.Messages[i].ID != messageID {
			continue
		}
		if status != "" {
			sess.Messages[i].Status = status
		}
		if strings.TrimSpace(text) != "" {
			sess.Messages[i].Text = strings.TrimSpace(text)
		}
		break
	}
	sess.LastSeen = time.Now().UnixMilli()
	_ = s.saveSessions()
}

func (s *mobileConnectStore) messages(sessionID string) []mobileChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return nil
	}
	out := make([]mobileChatMessage, len(sess.Messages))
	copy(out, sess.Messages)
	return out
}

func (a *App) GetMobileConnectConfig() MobileConnectConfig {
	if err := a.ensureClawBridge(); err != nil {
		return MobileConnectConfig{Enabled: true, Model: "deepseek-chat", Persona: "Be concise and practical."}
	}
	if a.clawBridge == nil || a.clawBridge.mobile == nil {
		return MobileConnectConfig{Enabled: true, Model: "deepseek-chat", Persona: "Be concise and practical."}
	}
	return a.clawBridge.mobile.getConfig()
}

func (a *App) RevokeMobileSession(sessionID string) error {
	if err := a.ensureClawBridge(); err != nil {
		return err
	}
	if a.clawBridge == nil || a.clawBridge.mobile == nil {
		return fmt.Errorf("mobile connect is not ready")
	}
	if !a.clawBridge.mobile.revokeSession(sessionID) {
		return fmt.Errorf("session not found")
	}
	a.clawBridge.emitMobileSessionsChanged()
	return nil
}

func (a *App) SaveMobileConnectConfig(cfg MobileConnectConfig) error {
	if err := a.ensureClawBridge(); err != nil {
		return err
	}
	if a.clawBridge == nil || a.clawBridge.mobile == nil {
		return fmt.Errorf("mobile connect is not ready")
	}
	prev := a.clawBridge.mobile.getConfig()
	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = prev.Model
	}
	if strings.TrimSpace(cfg.Persona) == "" {
		cfg.Persona = prev.Persona
	}
	if cfg.AllowLAN && !prev.AllowLAN {
		if !a.confirmAllowLAN() {
			return fmt.Errorf("cancelled")
		}
	}
	if !cfg.Enabled && prev.Enabled {
		a.clawBridge.mobile.revokeAllSessions()
	}
	if err := a.clawBridge.mobile.setConfig(cfg); err != nil {
		return err
	}
	if cfg.AllowLAN != prev.AllowLAN {
		if cfg.AllowLAN {
			ensureMobileLANFirewallRule(a.clawBridge.port)
		} else if a.clawBridge.tunnelPublicURL() == "" {
			a.clawBridge.mobile.clearActiveSessions()
		}
	} else if cfg.AllowLAN {
		ensureMobileLANFirewallRule(a.clawBridge.port)
	}
	a.clawBridge.stopRelayClient()
	a.clawBridge.startRelayClient()
	a.clawBridge.pushRelayPairToken()
	a.clawBridge.emitMobileSessionsChanged()
	return nil
}

func (a *App) GetMobilePairingInfo() MobilePairingInfo {
	if a == nil {
		return MobilePairingInfo{Port: defaultClawBridgePort, Enabled: true, BridgeReady: false, ConnectMode: "none"}
	}
	if err := a.ensureClawBridge(); err != nil {
		return MobilePairingInfo{
			Port: defaultClawBridgePort, Enabled: true, LanIP: primaryLANIP(),
			BridgeReady: false, ConnectMode: "none",
		}
	}
	if a.clawBridge == nil || a.clawBridge.mobile == nil {
		return MobilePairingInfo{Port: defaultClawBridgePort, Enabled: true, BridgeReady: false, ConnectMode: "none"}
	}
	return a.clawBridge.mobilePairingInfo()
}

func (a *App) RefreshMobilePairing() MobilePairingInfo {
	if a == nil {
		return MobilePairingInfo{Port: defaultClawBridgePort, Enabled: true, BridgeReady: false}
	}
	if err := a.ensureClawBridge(); err != nil {
		return MobilePairingInfo{Port: defaultClawBridgePort, Enabled: true, BridgeReady: false}
	}
	if a.clawBridge == nil || a.clawBridge.mobile == nil {
		return MobilePairingInfo{Port: defaultClawBridgePort, Enabled: true, BridgeReady: false}
	}
	info := a.clawBridge.mobile.refreshPairing(a.clawBridge.port, a.clawBridge.tunnelPublicURL(), a.clawBridge.relayConnected())
	info.BridgeReady = true
	a.clawBridge.pushRelayPairToken()
	return info
}

func (b *clawBridge) mobilePairingInfo() MobilePairingInfo {
	if b == nil || b.mobile == nil {
		return MobilePairingInfo{Port: defaultClawBridgePort, Enabled: true, BridgeReady: false}
	}
	info := b.mobile.pairingInfo(b.port, b.tunnelPublicURL(), b.relayConnected())
	info.BridgeReady = true
	return info
}

func (b *clawBridge) mobileRefreshPairing() MobilePairingInfo {
	if b == nil || b.mobile == nil {
		return MobilePairingInfo{Port: defaultClawBridgePort, Enabled: true}
	}
	info := b.mobile.refreshPairing(b.port, b.tunnelPublicURL(), b.relayConnected())
	b.pushRelayPairToken()
	return info
}

func (a *App) ListMobileSessions() []MobileSessionSummary {
	if a == nil {
		return nil
	}
	if err := a.ensureClawBridge(); err != nil {
		return nil
	}
	if a.clawBridge == nil || a.clawBridge.mobile == nil {
		return nil
	}
	return a.clawBridge.mobile.listSessions()
}

func (b *clawBridge) handleMobileHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = io.WriteString(w, `{"ok":true}`)
}

func (b *clawBridge) handleMobilePairPage(w http.ResponseWriter, r *http.Request) {
	if b == nil || b.mobile == nil {
		http.NotFound(w, r)
		return
	}
	b.touchTunnelActivity()
	token := strings.Trim(strings.TrimPrefix(r.URL.Path, "/mobile/p/"), "/")
	if token == "" {
		http.NotFound(w, r)
		return
	}
	page := strings.Replace(string(mobilePageHTML), "{{PAIR_TOKEN}}", jsonString(token), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, page)
}

func jsonString(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func (b *clawBridge) handleMobilePair(w http.ResponseWriter, r *http.Request) {
	if b == nil || b.mobile == nil {
		writeMobileJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "mobile connect unavailable"})
		return
	}
	b.touchTunnelActivity()
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if b.pairRL != nil && !b.pairRL.allow(clientIP(r)) {
		writeMobileJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many pairing attempts"})
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMobileJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	sess, err := b.mobile.pair(req.Token)
	if err != nil {
		writeMobileJSON(w, http.StatusUnauthorized, map[string]any{"error": err.Error()})
		return
	}
	writeMobileJSON(w, http.StatusOK, map[string]any{"sessionId": sess.ID, "ok": true})
	b.emitMobileSessionsChanged()
}

func (b *clawBridge) handleMobileMessages(w http.ResponseWriter, r *http.Request) {
	if b == nil || b.mobile == nil {
		writeMobileJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "mobile connect unavailable"})
		return
	}
	b.touchTunnelActivity()
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := strings.TrimSpace(r.URL.Query().Get("session"))
	if sessionID == "" {
		writeMobileJSON(w, http.StatusBadRequest, map[string]any{"error": "session is required"})
		return
	}
	if err := b.mobile.requirePairedSession(sessionID); err != nil {
		writeMobileJSON(w, http.StatusUnauthorized, map[string]any{"error": "session not found"})
		return
	}
	writeMobileJSON(w, http.StatusOK, map[string]any{"messages": b.mobile.messages(sessionID)})
}

func (b *clawBridge) handleMobileSend(w http.ResponseWriter, r *http.Request) {
	if b == nil || b.mobile == nil || b.app == nil {
		writeMobileJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "mobile connect unavailable"})
		return
	}
	b.touchTunnelActivity()
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SessionID string `json:"sessionId"`
		Text      string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMobileJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	req.SessionID = strings.TrimSpace(req.SessionID)
	if b.sessionRL != nil && !b.sessionRL.allow(clientIP(r)+"|"+req.SessionID) {
		writeMobileJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many requests"})
		return
	}
	pendingID, err := b.processMobileUserMessage(req.SessionID, req.Text)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errMobileSessionUnauthorized) {
			status = http.StatusUnauthorized
		}
		writeMobileJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeMobileJSON(w, http.StatusOK, map[string]any{"ok": true, "messageId": pendingID})
}

func (b *clawBridge) processMobileUserMessage(sessionID, text string) (string, error) {
	if b == nil || b.mobile == nil || b.app == nil {
		return "", fmt.Errorf("mobile connect unavailable")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", fmt.Errorf("session is required")
	}
	if err := b.mobile.requirePairedSession(sessionID); err != nil {
		return "", err
	}
	incoming, err := b.mobile.appendMessage(sessionID, text, false, "")
	if err != nil {
		return "", err
	}
	pending, err := b.mobile.appendMessage(sessionID, "思考中…", true, "pending")
	if err != nil {
		return "", err
	}
	b.emitMobileMessage(sessionID, incoming)
	b.emitMobileMessage(sessionID, pending)
	b.pushRelayMessages(sessionID)
	go b.runMobileAgentReply(sessionID, pending.ID, text)
	return pending.ID, nil
}

func (b *clawBridge) runMobileAgentReply(sessionID, pendingID, userText string) {
	if b == nil || b.app == nil || b.mobile == nil {
		return
	}
	ch := b.app.clawChannelFromActiveTab()
	reply, err := b.app.runClawAgentReply(ch, userText)
	if err != nil {
		reply = "处理失败：" + err.Error()
	}
	b.mobile.updateMessageStatus(sessionID, pendingID, "done", reply)
	patch := mobileChatMessage{ID: pendingID, Text: reply, Outgoing: true, Status: "done"}
	b.emitMobileMessage(sessionID, patch)
	b.pushRelayMessagePatch(sessionID, patch)
}

func (b *clawBridge) emitMobileMessage(sessionID string, msg mobileChatMessage) {
	if b == nil || b.app == nil || b.app.ctx == nil {
		return
	}
	runtime.EventsEmit(b.app.ctx, "mobile:message", map[string]any{
		"sessionId": sessionID,
		"message":   msg,
	})
}

func (b *clawBridge) emitMobileSessionsChanged() {
	if b == nil || b.app == nil || b.app.ctx == nil || b.mobile == nil {
		return
	}
	runtime.EventsEmit(b.app.ctx, "mobile:sessions", map[string]any{
		"activeCount": b.mobile.activeSessionCount(),
		"pairedCount": b.mobile.pairedCount(),
	})
}

func (b *clawBridge) disconnectRemoteSessions() {
	if b == nil || b.mobile == nil {
		return
	}
	if b.mobile.getConfig().AllowLAN {
		b.mobile.clearActiveSessions()
	} else {
		b.mobile.revokeAllSessions()
	}
	b.pushRelayPairToken()
	b.emitMobileSessionsChanged()
}

func writeMobileJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// lanBindIP returns the best-effort LAN IPv4 for the Connect UI (independent of AllowLAN toggle).
func lanBindIP() string {
	return primaryLANIP()
}

func isPrivateIPv4(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	switch {
	case v4[0] == 10:
		return true
	case v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31:
		return true
	case v4[0] == 192 && v4[1] == 168:
		return true
	default:
		return false
	}
}

func isLikelyVirtualInterface(name string) bool {
	lower := strings.ToLower(name)
	for _, token := range []string{
		"loopback", "virtual", "vethernet", "vmware", "virtualbox", "vbox",
		"hyper-v", "hyperv", "wsl", "docker", "vnic", "npcap", "tap", "tun",
		"bluetooth", "isatap", "teredo", "pseudo", "miniport",
	} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func outboundLANIP() string {
	for _, dest := range []string{"192.168.0.1:80", "192.168.1.1:80", "10.255.255.255:80", "8.8.8.8:80", "1.1.1.1:80"} {
		conn, err := net.Dial("udp4", dest)
		if err != nil {
			continue
		}
		addr, ok := conn.LocalAddr().(*net.UDPAddr)
		_ = conn.Close()
		if !ok || addr == nil {
			continue
		}
		ip := addr.IP.To4()
		if ip == nil || ip.IsLoopback() || !isPrivateIPv4(ip) {
			continue
		}
		if ip[0] == 169 && ip[1] == 254 {
			continue
		}
		return ip.String()
	}
	return ""
}

func primaryLANIP() string {
	if ip := outboundLANIP(); ip != "" {
		return ip
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	type candidate struct {
		ip    string
		score int
	}
	var best candidate
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if isLikelyVirtualInterface(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			v4 := ip.To4()
			if v4 == nil {
				continue
			}
			if v4[0] == 169 && v4[1] == 254 {
				continue
			}
			if !isPrivateIPv4(v4) {
				continue
			}
			score := 10
			name := strings.ToLower(iface.Name)
			if strings.Contains(name, "wi-fi") || strings.Contains(name, "wifi") || strings.Contains(name, "wlan") {
				score += 30
			}
			if strings.Contains(name, "ethernet") || strings.HasPrefix(name, "eth") {
				score += 25
			}
			if v4[0] == 192 && v4[1] == 168 {
				score += 15
			}
			if score > best.score {
				best = candidate{ip: v4.String(), score: score}
			}
		}
	}
	return best.ip
}

func randomToken(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("t-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// pairTokenEqual compares pairing tokens in constant time when lengths match.
func pairTokenEqual(stored, provided string) bool {
	a := []byte(stored)
	b := []byte(provided)
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}

func mobileQRDataURL(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	png, err := qrcode.Encode(text, qrcode.Medium, 256)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}
