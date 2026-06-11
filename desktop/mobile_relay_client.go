package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"arcdesk/internal/relay"
)

type mobileRelayClient struct {
	bridge    *clawBridge
	mu        sync.Mutex
	connected bool
	stop      chan struct{}
	send      chan []byte
}

func (b *clawBridge) startRelayClient() {
	if b == nil || b.mobile == nil {
		return
	}
	cfg := b.mobile.getConfig()
	if strings.TrimSpace(cfg.RelayBaseURL) == "" {
		return
	}
	b.ensureRelayDeviceCredentials()
	b.stopRelayClient()
	client := &mobileRelayClient{bridge: b, stop: make(chan struct{}), send: make(chan []byte, 64)}
	b.relay = client
	go client.loop()
}

func (b *clawBridge) stopRelayClient() {
	if b == nil || b.relay == nil {
		return
	}
	close(b.relay.stop)
	b.relay = nil
}

func (b *clawBridge) ensureRelayDeviceCredentials() {
	if b == nil || b.mobile == nil {
		return
	}
	cfg := b.mobile.getConfig()
	if strings.TrimSpace(cfg.DeviceID) != "" && strings.TrimSpace(cfg.DeviceSecret) != "" {
		return
	}
	cfg.DeviceID = randomToken(12)
	cfg.DeviceSecret = randomToken(24)
	_ = b.mobile.setConfig(cfg)
}

func (b *clawBridge) relayConnected() bool {
	if b == nil || b.relay == nil {
		return false
	}
	b.relay.mu.Lock()
	defer b.relay.mu.Unlock()
	return b.relay.connected
}

func (b *clawBridge) pushRelayPairToken() {
	if b == nil || b.relay == nil || b.mobile == nil {
		return
	}
	token, expiresAt := b.mobile.currentPairToken()
	frame, _ := json.Marshal(relay.PairToken{
		Type:      relay.MsgPairToken,
		PairToken: token,
		ExpiresAt: expiresAt,
	})
	b.relay.enqueue(frame)
}

func (b *clawBridge) pushRelayMessages(sessionID string) {
	if b == nil || b.relay == nil || b.mobile == nil {
		return
	}
	msgs := b.mobile.messages(sessionID)
	relayMsgs := make([]relay.ChatMessage, 0, len(msgs))
	for _, msg := range msgs {
		relayMsgs = append(relayMsgs, toRelayChatMessage(msg))
	}
	frame, _ := json.Marshal(relay.Messages{
		Type:      relay.MsgMessages,
		SessionID: sessionID,
		Messages:  relayMsgs,
	})
	b.relay.enqueue(frame)
}

func (b *clawBridge) pushRelayMessagePatch(sessionID string, msg mobileChatMessage) {
	if b == nil || b.relay == nil {
		return
	}
	frame, _ := json.Marshal(relay.MessagePatch{
		Type:      relay.MsgMessagePatch,
		SessionID: sessionID,
		Message:   toRelayChatMessage(msg),
	})
	b.relay.enqueue(frame)
}

func toRelayChatMessage(msg mobileChatMessage) relay.ChatMessage {
	return relay.ChatMessage{
		ID:        msg.ID,
		Text:      msg.Text,
		Outgoing:  msg.Outgoing,
		Status:    msg.Status,
		CreatedAt: msg.CreatedAt,
	}
}

func (c *mobileRelayClient) enqueue(raw []byte) {
	if c == nil {
		return
	}
	select {
	case c.send <- raw:
	default:
	}
}

func (c *mobileRelayClient) setConnected(v bool) {
	c.mu.Lock()
	c.connected = v
	c.mu.Unlock()
}

func (c *mobileRelayClient) loop() {
	backoff := time.Second
	for {
		select {
		case <-c.stop:
			c.setConnected(false)
			return
		default:
		}
		if err := c.connectOnce(); err != nil {
			slog.Warn("mobile relay connect", "err", err)
			c.setConnected(false)
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
	}
}

func (c *mobileRelayClient) connectOnce() error {
	if c == nil || c.bridge == nil || c.bridge.mobile == nil {
		return nil
	}
	cfg := c.bridge.mobile.getConfig()
	wsURL, err := relayWebSocketURL(cfg.RelayBaseURL)
	if err != nil {
		return err
	}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	c.setConnected(true)
	defer c.setConnected(false)

	token, expiresAt := c.bridge.mobile.currentPairToken()
	hello, _ := json.Marshal(relay.Hello{
		Type:      relay.MsgHello,
		DeviceID:  cfg.DeviceID,
		Secret:    cfg.DeviceSecret,
		PairToken: token,
		ExpiresAt: expiresAt,
	})
	if err := conn.WriteMessage(websocket.TextMessage, hello); err != nil {
		return err
	}

	errCh := make(chan error, 2)
	stop := make(chan struct{})
	defer close(stop)

	go func() {
		for {
			select {
			case <-stop:
				return
			case raw := <-c.send:
				if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	go func() {
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			c.handleFrame(raw)
		}
	}()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.stop:
			return nil
		case err := <-errCh:
			return err
		case <-ticker.C:
			c.bridge.pushRelayPairToken()
		}
	}
}

func (c *mobileRelayClient) handleFrame(raw []byte) {
	var kind struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &kind) != nil {
		return
	}
	if kind.Type != relay.MsgSend || c.bridge == nil {
		return
	}
	var msg relay.Send
	if json.Unmarshal(raw, &msg) != nil {
		return
	}
	_, _ = c.bridge.processMobileUserMessage(msg.SessionID, msg.Text)
}

func relayWebSocketURL(base string) (string, error) {
	raw := strings.TrimSpace(base)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	host := strings.ToLower(u.Hostname())
	loopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	switch strings.ToLower(u.Scheme) {
	case "https":
		u.Scheme = "wss"
	case "http":
		if !loopback {
			return "", fmt.Errorf("relay URL must use https for remote hosts")
		}
		u.Scheme = "ws"
	case "ws":
		if !loopback {
			return "", fmt.Errorf("relay URL must use wss for remote hosts")
		}
	case "wss":
	default:
		if !loopback {
			return "", fmt.Errorf("relay URL must use https for remote hosts")
		}
		u.Scheme = "ws"
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/ws/desktop"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
