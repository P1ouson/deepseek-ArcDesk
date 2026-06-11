package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"arcdesk/internal/channels/wecom"
)

const (
	defaultClawBridgePort   = 8787
	clawBridgeBindLocalhost = "127.0.0.1"
	clawBridgeBindLAN       = "0.0.0.0"
)

type clawBridge struct {
	app       *App
	mu        sync.Mutex
	srv       *http.Server
	port      int
	mobile    *mobileConnectStore
	relay     *mobileRelayClient
	tunnel    *mobileTunnelRunner
	pairRL    *pairRateLimiter
	sessionRL *actionRateLimiter
}

type ClawCallbackInfo struct {
	BaseURL string `json:"baseUrl"`
	Path    string `json:"path"`
	URL     string `json:"url"`
	Port    int    `json:"port"`
}

func (a *App) startClawBridge() {
	if a == nil {
		return
	}
	a.clawBridgeMu.Lock()
	defer a.clawBridgeMu.Unlock()
	if a.clawBridge != nil {
		return
	}
	a.clawBridge = &clawBridge{
		app:       a,
		port:      defaultClawBridgePort,
		mobile:    newMobileConnectStore(),
		pairRL:    newPairRateLimiter(),
		sessionRL: newSessionRateLimiter(),
	}
	if err := a.clawBridge.start(); err != nil {
		slog.Warn("claw bridge failed to start", "err", err)
		a.clawBridge = nil
	}
}

// ensureClawBridge lazily starts the mobile/claw HTTP bridge when remote features are used.
func (a *App) ensureClawBridge() error {
	if a == nil {
		return fmt.Errorf("app unavailable")
	}
	a.clawBridgeMu.Lock()
	ready := a.clawBridge != nil
	a.clawBridgeMu.Unlock()
	if ready {
		return nil
	}
	a.startClawBridge()
	a.clawBridgeMu.Lock()
	defer a.clawBridgeMu.Unlock()
	if a.clawBridge == nil {
		return fmt.Errorf("mobile connect is not ready")
	}
	return nil
}

func (a *App) stopClawBridge() {
	if a == nil || a.clawBridge == nil {
		return
	}
	a.clawBridge.stop()
	a.clawBridge = nil
}

func clawBridgeBindHost(_ bool) string {
	// Always bind all interfaces; non-loopback access is gated in middleware
	// so toggling AllowLAN does not require restarting the listener.
	return clawBridgeBindLAN
}

func (b *clawBridge) bindHost() string {
	return clawBridgeBindLAN
}

func isLoopbackIP(ip string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "localhost" || ip == "127.0.0.1" || ip == "::1" {
		return true
	}
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}

func (b *clawBridge) mobileAccessAllowed(r *http.Request) bool {
	if b == nil || b.mobile == nil {
		return false
	}
	if b.mobile.getConfig().AllowLAN {
		return true
	}
	return isLoopbackIP(clientIP(r))
}

func (b *clawBridge) withMobileAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !b.mobileAccessAllowed(r) {
			http.Error(w, "LAN access disabled", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func clawBridgeListenAddr(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

func cloudflaredTunnelTarget(port int) string {
	return fmt.Sprintf("http://%s:%d", clawBridgeBindLocalhost, port)
}

func (b *clawBridge) newHTTPServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/claw/wecom/", b.handleWeCom)
	mux.HandleFunc("/mobile/health", b.withMobileAccess(b.handleMobileHealth))
	mux.HandleFunc("/mobile/p/", b.withMobileAccess(b.handleMobilePairPage))
	mux.HandleFunc("/mobile/api/pair", b.withMobileAccess(b.handleMobilePair))
	mux.HandleFunc("/mobile/api/messages", b.withMobileAccess(b.handleMobileMessages))
	mux.HandleFunc("/mobile/api/send", b.withMobileAccess(b.handleMobileSend))
	mux.HandleFunc("/mobile/api/decision", b.withMobileAccess(b.handleMobileDecision))
	return &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
}

func (b *clawBridge) startListenerLocked() error {
	addr := clawBridgeListenAddr(b.bindHost(), b.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	b.srv = b.newHTTPServer()
	if b.bindHost() == clawBridgeBindLAN {
		ensureMobileLANFirewallRule(b.port)
	}
	slog.Info("claw bridge listening", "addr", addr)
	go func() {
		if err := b.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Warn("claw bridge stopped", "err", err)
		}
	}()
	return nil
}

func (b *clawBridge) startListener() error {
	if b == nil {
		return fmt.Errorf("claw bridge unavailable")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.startListenerLocked()
}

func (b *clawBridge) start() error {
	if err := b.startListener(); err != nil {
		return err
	}
	if b.mobile != nil && b.mobile.getConfig().AllowLAN {
		ensureMobileLANFirewallRule(b.port)
	}
	b.startRelayClient()
	return nil
}

func (b *clawBridge) stop() {
	if b == nil {
		return
	}
	b.stopRelayClient()
	b.stopMobileTunnel()
	if b.srv == nil {
		return
	}
	ctx, cancel := contextWithTimeout(5 * time.Second)
	defer cancel()
	_ = b.srv.Shutdown(ctx)
}

func (b *clawBridge) baseURL() string {
	if b == nil || b.port <= 0 {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d", b.port)
}

func (b *clawBridge) callbackInfo(channelID string) ClawCallbackInfo {
	path := "/claw/wecom/" + strings.TrimSpace(channelID)
	base := b.baseURL()
	return ClawCallbackInfo{
		BaseURL: base,
		Path:    path,
		URL:     base + path,
		Port:    b.port,
	}
}

func (a *App) GetClawCallbackInfo(channelID string) ClawCallbackInfo {
	if a == nil || a.clawBridge == nil {
		return ClawCallbackInfo{Port: defaultClawBridgePort, Path: "/claw/wecom/" + strings.TrimSpace(channelID)}
	}
	return a.clawBridge.callbackInfo(channelID)
}

func (a *App) TestClawWeComChannel(channel ClawChannel) string {
	client, err := wecomClientFromChannel(channel)
	if err != nil {
		return err.Error()
	}
	if err := client.Ping(); err != nil {
		return err.Error()
	}
	if _, err := wecom.NewCrypt(channel.WeComToken, channel.WeComEncodingAESKey, channel.WeComCorpID); err != nil {
		return err.Error()
	}
	return ""
}

func (b *clawBridge) handleWeCom(w http.ResponseWriter, r *http.Request) {
	if b != nil && b.app != nil {
		_ = b.app.ensureClawBridge()
	}
	channelID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/claw/wecom/"), "/")
	if channelID == "" {
		http.NotFound(w, r)
		return
	}
	ch, ok := loadClawChannelByID(channelID)
	if !ok || ch.Type != "wechat" || !ch.Enabled {
		http.NotFound(w, r)
		return
	}
	crypt, err := wecom.NewCrypt(ch.WeComToken, ch.WeComEncodingAESKey, ch.WeComCorpID)
	if err != nil {
		http.Error(w, "invalid channel crypto config", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		echo, err := crypt.VerifyURL(r.URL.Query().Get("msg_signature"), r.URL.Query().Get("timestamp"), r.URL.Query().Get("nonce"), r.URL.Query().Get("echostr"))
		if err != nil {
			http.Error(w, "verify failed", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, echo)
	case http.MethodPost:
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read body failed", http.StatusBadRequest)
			return
		}
		encrypted, err := wecom.ParseEncryptedEnvelope(body)
		if err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		plainXML, err := crypt.DecryptMsg(r.URL.Query().Get("msg_signature"), r.URL.Query().Get("timestamp"), r.URL.Query().Get("nonce"), encrypted)
		if err != nil {
			http.Error(w, "decrypt failed", http.StatusBadRequest)
			return
		}
		msg, err := wecom.ParseIncomingMessage([]byte(plainXML))
		if err != nil {
			http.Error(w, "parse failed", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "success")
		if strings.EqualFold(msg.MsgType, "text") && strings.TrimSpace(msg.Content) != "" {
			go b.handleWeComText(ch, msg)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (b *clawBridge) handleWeComText(ch ClawChannel, msg wecom.IncomingMessage) {
	if b == nil || b.app == nil {
		return
	}
	incoming, err := appendClawMessage(ch.ID, msg.Content, false)
	if err != nil {
		slog.Warn("claw wecom store incoming", "err", err)
		return
	}
	b.emitClawMessage(ch.ID, incoming)

	reply, err := b.app.runClawAgentReply(ch, msg.Content)
	if err != nil {
		slog.Warn("claw wecom agent reply", "err", err)
		reply = "处理失败：" + err.Error()
	}
	outgoing, err := appendClawMessage(ch.ID, reply, true)
	if err != nil {
		slog.Warn("claw wecom store outgoing", "err", err)
	} else {
		b.emitClawMessage(ch.ID, outgoing)
	}
	client, err := wecomClientFromChannel(ch)
	if err != nil {
		slog.Warn("claw wecom send", "err", err)
		return
	}
	if err := client.SendText(msg.FromUserName, reply); err != nil {
		slog.Warn("claw wecom send", "err", err)
	}
}

func (b *clawBridge) emitClawMessage(channelID string, msg ClawMessage) {
	if b == nil || b.app == nil || b.app.ctx == nil {
		return
	}
	runtime.EventsEmit(b.app.ctx, "claw:message", map[string]any{
		"channelId": channelID,
		"message":   msg,
	})
}

func validateClawChannel(channel ClawChannel) error {
	switch strings.TrimSpace(channel.Type) {
	case "wechat":
		if strings.TrimSpace(channel.WeComCorpID) == "" {
			return fmt.Errorf("wecom corp id is required")
		}
		if strings.TrimSpace(channel.WeComAgentID) == "" {
			return fmt.Errorf("wecom agent id is required")
		}
		if strings.TrimSpace(channel.WeComSecret) == "" {
			return fmt.Errorf("wecom secret is required")
		}
		if strings.TrimSpace(channel.WeComToken) == "" {
			return fmt.Errorf("wecom callback token is required")
		}
		if strings.TrimSpace(channel.WeComEncodingAESKey) == "" {
			return fmt.Errorf("wecom encoding aes key is required")
		}
		if _, err := wecom.NewCrypt(channel.WeComToken, channel.WeComEncodingAESKey, channel.WeComCorpID); err != nil {
			return err
		}
	case "webhook":
		// optional URL until send time
	}
	return nil
}

func loadClawChannelByID(id string) (ClawChannel, bool) {
	key := strings.TrimSpace(id)
	if key == "" {
		return ClawChannel{}, false
	}
	items, err := loadClawChannels(ARCDESKDesktopDataPath("claw-channels.json"))
	if err != nil {
		return ClawChannel{}, false
	}
	for _, item := range items {
		if item.ID == key {
			return item, true
		}
	}
	return ClawChannel{}, false
}

func wecomClientFromChannel(ch ClawChannel) (*wecom.Client, error) {
	return wecom.NewClient(ch.WeComCorpID, ch.WeComSecret, ch.WeComAgentID)
}

func appendClawMessage(channelID, text string, outgoing bool) (ClawMessage, error) {
	key := strings.TrimSpace(channelID)
	body := strings.TrimSpace(text)
	if key == "" {
		return ClawMessage{}, fmt.Errorf("channel id is required")
	}
	if body == "" {
		return ClawMessage{}, fmt.Errorf("message text is required")
	}
	msg := ClawMessage{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		ChannelID: key,
		Text:      body,
		Outgoing:  outgoing,
		CreatedAt: time.Now().UnixMilli(),
	}
	path := ARCDESKDesktopDataPath("claw-messages.json")
	items, _ := loadClawMessages(path)
	items = append(items, msg)
	if err := saveSensitiveJSON(path, items); err != nil {
		return ClawMessage{}, err
	}
	return msg, nil
}

func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
