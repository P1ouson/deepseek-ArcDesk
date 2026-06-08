package relay

// Wire message types between desktop and relay.
const (
	MsgHello        = "hello"
	MsgPairToken    = "pair_token"
	MsgSend         = "send"
	MsgMessages     = "messages"
	MsgMessagePatch = "message_patch"
)

// Hello is sent when a desktop connects to the relay.
type Hello struct {
	Type      string `json:"type"`
	DeviceID  string `json:"deviceId"`
	Secret    string `json:"secret"`
	PairToken string `json:"pairToken"`
	ExpiresAt int64  `json:"expiresAt"`
}

// PairToken updates the active pairing QR token for a desktop.
type PairToken struct {
	Type      string `json:"type"`
	PairToken string `json:"pairToken"`
	ExpiresAt int64  `json:"expiresAt"`
}

// Send is relay → desktop: phone sent a message.
type Send struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	RequestID string `json:"requestId"`
	Text      string `json:"text"`
}

// ChatMessage is a single mobile chat line.
type ChatMessage struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Outgoing  bool   `json:"outgoing"`
	Status    string `json:"status,omitempty"`
	CreatedAt int64  `json:"createdAt"`
}

// Messages replaces the full message list for a session (desktop → relay).
type Messages struct {
	Type      string        `json:"type"`
	SessionID string        `json:"sessionId"`
	Messages  []ChatMessage `json:"messages"`
}

// MessagePatch updates one message in a session (desktop → relay).
type MessagePatch struct {
	Type      string      `json:"type"`
	SessionID string      `json:"sessionId"`
	Message   ChatMessage `json:"message"`
}
