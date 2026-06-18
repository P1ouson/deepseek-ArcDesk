package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"arcdesk/internal/agent"
	"arcdesk/internal/i18n"
	"arcdesk/internal/provider"
)

var (
	providerTransportLine = regexp.MustCompile(`(?i)^[\w.-]+:\s*(?:request failed|read stream|build request|decode stream|marshal request):\s*`)
	providerBareMessage   = regexp.MustCompile(`(?i)^[\w.-]+:\s*(.+)$`)
	providerStatusLine    = regexp.MustCompile(`(?i)^[\w.-]+:\s*status\s*(\d{3})(?::\s*(.*))?$`)
)

var operationalPrefixes = []string{
	"fetch models:",
	"validate:",
	"config:",
	"rebuild:",
	"network:",
	"save:",
	"cannot switch model:",
}

// ExplainError maps provider and transport failures to actionable, localized
// messages for UI surfaces (turn_done, settings, onboarding, startup).
func ExplainError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, agent.ErrEmptyModelResponse) {
		return errors.New(i18n.M.AgentEmptyResponse)
	}
	var apiErr *provider.APIError
	if errors.As(err, &apiErr) {
		msg := i18n.M.ProviderStatusMessage(apiErr.Status)
		if msg == "" {
			return err
		}
		if reason := requestErrorReason(apiErr); reason != "" {
			return fmt.Errorf("%s\n%s", msg, reason)
		}
		return errors.New(msg)
	}
	var authErr *provider.AuthError
	if errors.As(err, &authErr) {
		msg := i18n.M.ProviderStatusMessage(authErr.Status)
		if msg == "" {
			return err
		}
		if authErr.KeyEnv != "" {
			return fmt.Errorf("%s (%s)", msg, authErr.KeyEnv)
		}
		return errors.New(msg)
	}
	if msg := humanizeTransportError(err); msg != "" {
		return errors.New(msg)
	}
	return err
}

// UserFacingMessage is ExplainError(err).Error() with an empty string for nil.
func UserFacingMessage(err error) string {
	if err == nil {
		return ""
	}
	return ExplainError(err).Error()
}

func humanizeTransportError(err error) string {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if errors.Is(e, context.Canceled) {
			return i18n.M.AgentRequestCanceled
		}
		if errors.Is(e, context.DeadlineExceeded) {
			return i18n.M.AgentRequestTimeout
		}
	}

	raw := strings.TrimSpace(err.Error())
	raw = stripOperationalPrefixes(raw)
	if msg := humanizeStatusLine(raw); msg != "" {
		return msg
	}

	tail := providerTransportLine.ReplaceAllString(raw, "")
	hadProviderPrefix := tail != raw
	tail = strings.TrimSpace(strings.TrimPrefix(tail, "request failed:"))
	tail = strings.TrimSpace(strings.TrimPrefix(tail, "read stream:"))
	tail = strings.TrimSpace(strings.TrimPrefix(tail, "decode stream:"))
	tail = strings.TrimSpace(stripHTTPMethodURL(tail))

	if msg := classifyTransportTail(tail); msg != "" {
		return msg
	}
	if hadProviderPrefix && tail != "" {
		return tail
	}

	if m := providerBareMessage.FindStringSubmatch(raw); m != nil {
		body := strings.TrimSpace(m[1])
		if !isProviderOp(body) {
			if msg := classifyTransportTail(body); msg != "" {
				return msg
			}
			if msg := humanizeStatusLine(body); msg != "" {
				return msg
			}
			return body
		}
	}
	return ""
}

func stripOperationalPrefixes(raw string) string {
	out := strings.TrimSpace(raw)
	for {
		changed := false
		for _, prefix := range operationalPrefixes {
			if len(out) >= len(prefix) && strings.EqualFold(out[:len(prefix)], prefix) {
				out = strings.TrimSpace(out[len(prefix):])
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return out
}

func humanizeStatusLine(raw string) string {
	m := providerStatusLine.FindStringSubmatch(raw)
	if m == nil {
		return ""
	}
	code, err := strconv.Atoi(m[1])
	if err != nil {
		return ""
	}
	msg := i18n.M.ProviderStatusMessage(code)
	if msg == "" {
		return ""
	}
	body := strings.TrimSpace(m[2])
	if body != "" && (code == 400 || code == 422) {
		if reason := providerBodyReason(body); reason != "" {
			return msg + "\n" + reason
		}
	}
	return msg
}

func isProviderOp(body string) bool {
	lower := strings.ToLower(body)
	for _, prefix := range []string{
		"request failed:",
		"read stream:",
		"build request:",
		"decode stream:",
		"marshal request:",
		"status ",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func classifyTransportTail(tail string) string {
	if tail == "" {
		return ""
	}
	lower := strings.ToLower(tail)
	switch {
	case strings.Contains(lower, "context canceled"):
		return i18n.M.AgentRequestCanceled
	case strings.Contains(lower, "context deadline exceeded"):
		return i18n.M.AgentRequestTimeout
	case strings.Contains(lower, "unexpected eof"):
		return i18n.M.AgentStreamInterrupted
	case isNetworkFailure(lower):
		return i18n.M.AgentNetworkError
	case strings.Contains(lower, "timeout"):
		return i18n.M.AgentRequestTimeout
	default:
		return ""
	}
}

func isNetworkFailure(lower string) bool {
	for _, needle := range []string{
		"eof",
		"connection reset",
		"connection refused",
		"broken pipe",
		"tls:",
		"dial tcp",
		"i/o timeout",
		"network is unreachable",
		"no such host",
		"wsarecv",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func stripHTTPMethodURL(s string) string {
	for _, method := range []string{"Post ", "Get ", "Put ", "Patch ", "Delete "} {
		if !strings.HasPrefix(s, method) {
			continue
		}
		rest := s[len(method):]
		if !strings.HasPrefix(rest, `"`) {
			continue
		}
		close := strings.Index(rest[1:], `"`)
		if close < 0 {
			continue
		}
		after := strings.TrimSpace(rest[close+2:])
		if strings.HasPrefix(after, ":") {
			return strings.TrimSpace(after[1:])
		}
	}
	return s
}

// requestErrorReason returns the provider's verbatim reason for request-shaped
// 4xx (400/422) — the localized line names the category, the body names the
// actual cause (context-length exceeded, unpaired tool_calls). Empty otherwise.
func requestErrorReason(e *provider.APIError) string {
	if e.Status != 400 && e.Status != 422 {
		return ""
	}
	return providerBodyReason(e.Body)
}

// providerBodyReason pulls the human reason from an OpenAI/Anthropic-shaped error
// body ({"error":{"message":…}}), falling back to the trimmed raw body.
func providerBodyReason(body string) string {
	if body == "" {
		return ""
	}
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(body), &parsed) == nil && parsed.Error.Message != "" {
		return clampRunes(parsed.Error.Message, 800)
	}
	return clampRunes(body, 800)
}

func clampRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
