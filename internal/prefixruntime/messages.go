package prefixruntime

import (
	"strings"

	"arcdesk/internal/provider"
)

// systemBlockSep joins frozen system-zone blocks with a stable delimiter so
// byte-identical prefixes survive across turns (DeepSeek prefix-exact cache).
const systemBlockSep = "\n\n"

// ForProviderRequest returns a copy of msgs with Zone A (system) canonicalized:
// all system-role content is merged into a single message at index 0; the tail
// (user/assistant/tool) stays append-only in original order.
func ForProviderRequest(msgs []provider.Message) []provider.Message {
	if len(msgs) == 0 {
		return nil
	}
	var frozen []string
	var tail []provider.Message
	for _, m := range msgs {
		if m.Role == provider.RoleSystem {
			if c := strings.TrimSpace(m.Content); c != "" {
				frozen = append(frozen, c)
			}
			continue
		}
		tail = append(tail, m)
	}
	if len(frozen) == 0 {
		return append([]provider.Message(nil), msgs...)
	}
	out := make([]provider.Message, 0, 1+len(tail))
	out = append(out, provider.Message{
		Role:    provider.RoleSystem,
		Content: strings.Join(frozen, systemBlockSep),
	})
	out = append(out, tail...)
	return out
}

// FrozenSystemText returns the merged Zone A text (empty when none).
func FrozenSystemText(msgs []provider.Message) string {
	var parts []string
	for _, m := range msgs {
		if m.Role != provider.RoleSystem {
			continue
		}
		if c := strings.TrimSpace(m.Content); c != "" {
			parts = append(parts, c)
		}
	}
	return strings.Join(parts, systemBlockSep)
}

// TailMessageCount is the number of non-system messages (Zone C append tail).
func TailMessageCount(msgs []provider.Message) int {
	n := 0
	for _, m := range msgs {
		if m.Role != provider.RoleSystem {
			n++
		}
	}
	return n
}
