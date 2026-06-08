package relay

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed mobile_page.html
var mobilePageHTML []byte

const (
	defaultRequestTimeout = 90 * time.Second
	writeWait             = 10 * time.Second
	pongWait              = 60 * time.Second
	pingPeriod            = 45 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server is the cloud relay that bridges phone HTTP clients and desktop websockets.
type Server struct {
	mu         sync.RWMutex
	devices    map[string]*desktopLink
	secrets    map[string]string
	pairTokens map[string]pairBinding
	sessions   map[string]*relaySession
}

type pairBinding struct {
	deviceID  string
	expiresAt int64
}

type relaySession struct {
	id        string
	deviceID  string
	createdAt int64
	lastSeen  int64
	messages  []ChatMessage
}

type desktopLink struct {
	deviceID string
	conn     *websocket.Conn
	send     chan []byte
}

// NewServer constructs an empty relay server.
func NewServer() *Server {
	return &Server{
		devices:    map[string]*desktopLink{},
		secrets:    map[string]string{},
		pairTokens: map[string]pairBinding{},
		sessions:   map[string]*relaySession{},
	}
}

// Handler returns the relay HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/desktop", s.handleDesktopWS)
	mux.HandleFunc("/mobile/p/", s.handleMobilePage)
	mux.HandleFunc("/mobile/api/pair", s.handlePair)
	mux.HandleFunc("/mobile/api/messages", s.handleMessages)
	mux.HandleFunc("/mobile/api/send", s.handleSend)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	return mux
}

// ListenAndServe starts the relay HTTP server.
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

func (s *Server) handleDesktopWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	link := &desktopLink{conn: conn, send: make(chan []byte, 32)}
	defer func() {
		conn.Close()
		if link.deviceID != "" {
			s.mu.Lock()
			if cur, ok := s.devices[link.deviceID]; ok && cur.conn == conn {
				delete(s.devices, link.deviceID)
			}
			s.mu.Unlock()
		}
	}()

	go s.desktopWriter(link)
	s.desktopReader(link)
}

func (s *Server) desktopWriter(link *desktopLink) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-link.send:
			_ = link.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = link.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := link.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = link.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := link.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) desktopReader(link *desktopLink) {
	link.conn.SetReadLimit(1 << 20)
	_ = link.conn.SetReadDeadline(time.Now().Add(pongWait))
	link.conn.SetPongHandler(func(string) error {
		return link.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, raw, err := link.conn.ReadMessage()
		if err != nil {
			return
		}
		s.handleDesktopFrame(link, raw)
	}
}

func (s *Server) handleDesktopFrame(link *desktopLink, raw []byte) {
	var kind struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &kind); err != nil {
		return
	}
	switch kind.Type {
	case MsgHello:
		var msg Hello
		if json.Unmarshal(raw, &msg) != nil {
			return
		}
		deviceID := strings.TrimSpace(msg.DeviceID)
		secret := strings.TrimSpace(msg.Secret)
		if deviceID == "" || secret == "" {
			return
		}
		s.mu.Lock()
		if existing, ok := s.secrets[deviceID]; ok && existing != secret {
			s.mu.Unlock()
			return
		}
		s.secrets[deviceID] = secret
		if old, ok := s.devices[deviceID]; ok && old.conn != link.conn {
			_ = old.conn.Close()
		}
		link.deviceID = deviceID
		s.devices[deviceID] = link
		s.bindPairTokenLocked(deviceID, msg.PairToken, msg.ExpiresAt)
		s.mu.Unlock()
	case MsgPairToken:
		var msg PairToken
		if json.Unmarshal(raw, &msg) != nil {
			return
		}
		if link.deviceID == "" {
			return
		}
		s.mu.Lock()
		s.bindPairTokenLocked(link.deviceID, msg.PairToken, msg.ExpiresAt)
		s.mu.Unlock()
	case MsgMessages:
		var msg Messages
		if json.Unmarshal(raw, &msg) != nil {
			return
		}
		s.mu.Lock()
		if sess, ok := s.sessions[msg.SessionID]; ok && sess.deviceID == link.deviceID {
			sess.messages = append([]ChatMessage(nil), msg.Messages...)
			sess.lastSeen = time.Now().UnixMilli()
		}
		s.mu.Unlock()
	case MsgMessagePatch:
		var msg MessagePatch
		if json.Unmarshal(raw, &msg) != nil {
			return
		}
		s.mu.Lock()
		if sess, ok := s.sessions[msg.SessionID]; ok && sess.deviceID == link.deviceID {
			updated := false
			for i := range sess.messages {
				if sess.messages[i].ID == msg.Message.ID {
					sess.messages[i] = msg.Message
					updated = true
					break
				}
			}
			if !updated {
				sess.messages = append(sess.messages, msg.Message)
			}
			sess.lastSeen = time.Now().UnixMilli()
		}
		s.mu.Unlock()
	}
}

func (s *Server) bindPairTokenLocked(deviceID, token string, expiresAt int64) {
	token = strings.TrimSpace(token)
	deviceID = strings.TrimSpace(deviceID)
	if token == "" || deviceID == "" {
		return
	}
	s.pairTokens[token] = pairBinding{deviceID: deviceID, expiresAt: expiresAt}
}

func (s *Server) handleMobilePage(w http.ResponseWriter, r *http.Request) {
	token := strings.Trim(strings.TrimPrefix(r.URL.Path, "/mobile/p/"), "/")
	if token == "" {
		http.NotFound(w, r)
		return
	}
	page := strings.Replace(string(mobilePageHTML), "{{PAIR_TOKEN}}", mustJSONString(token), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, page)
}

func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	deviceID, err := s.deviceForPairToken(req.Token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": err.Error()})
		return
	}
	sess := s.createSession(deviceID)
	writeJSON(w, http.StatusOK, map[string]any{"sessionId": sess.id, "ok": true})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := strings.TrimSpace(r.URL.Query().Get("session"))
	s.mu.RLock()
	sess, ok := s.sessions[sessionID]
	var msgs []ChatMessage
	if ok {
		msgs = append([]ChatMessage(nil), sess.messages...)
		sess.lastSeen = time.Now().UnixMilli()
	}
	s.mu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SessionID string `json:"sessionId"`
		Text      string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "text is required"})
		return
	}
	s.mu.RLock()
	sess, ok := s.sessions[strings.TrimSpace(req.SessionID)]
	var link *desktopLink
	if ok {
		link = s.devices[sess.deviceID]
	}
	s.mu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "session not found"})
		return
	}
	if link == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "desktop offline"})
		return
	}
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	payload, _ := json.Marshal(Send{
		Type:      MsgSend,
		SessionID: sess.id,
		RequestID: requestID,
		Text:      text,
	})
	select {
	case link.send <- payload:
	default:
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "desktop busy"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "requestId": requestID})
}

func (s *Server) deviceForPairToken(token string) (string, error) {
	key := strings.TrimSpace(token)
	if key == "" {
		return "", fmt.Errorf("pairing token is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	bind, ok := s.pairTokens[key]
	if !ok || time.Now().UnixMilli() > bind.expiresAt {
		return "", fmt.Errorf("pairing token expired or invalid")
	}
	if _, online := s.devices[bind.deviceID]; !online {
		return "", fmt.Errorf("desktop offline")
	}
	return bind.deviceID, nil
}

func (s *Server) createSession(deviceID string) *relaySession {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := &relaySession{
		id:        randomID("sess"),
		deviceID:  deviceID,
		createdAt: time.Now().UnixMilli(),
		lastSeen:  time.Now().UnixMilli(),
	}
	s.sessions[sess.id] = sess
	return sess
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func mustJSONString(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func randomID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// Run starts the relay until ctx is cancelled.
func Run(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           NewServer().Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("arcdesk relay listening", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
