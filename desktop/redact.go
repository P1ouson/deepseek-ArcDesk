package main

import (
	"regexp"
	"strings"
)

var (
	redactBearerToken = regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9._\-+/=]{8,}`)
	redactAPIKeyValue = regexp.MustCompile(`(?i)((?:api[_-]?key|token|secret|password|authorization)\s*[:=]\s*["']?)[^\s"',;]+`)
	redactSKPrefix    = regexp.MustCompile(`sk-[A-Za-z0-9._\-]{8,}`)
	redactTunnelURL   = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.(trycloudflare|cfargotunnel)\.com`)
	redactMobilePath  = regexp.MustCompile(`/mobile/p/[A-Za-z0-9]+`)
	redactSessionQuery = regexp.MustCompile(`(?i)([?&]session=)[^&\s"']+`)
)

// redactSecrets masks common credential patterns before values reach logs or notices.
func redactSecrets(s string) string {
	if s == "" {
		return s
	}
	out := redactBearerToken.ReplaceAllString(s, `${1}[REDACTED]`)
	out = redactAPIKeyValue.ReplaceAllString(out, `${1}[REDACTED]`)
	out = redactSKPrefix.ReplaceAllString(out, `sk-[REDACTED]`)
	return out
}

// redactURLQuery redacts sensitive query parameters in URLs (e.g. pairing tokens).
func redactURLQuery(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	out := redactTunnelURL.ReplaceAllString(raw, "https://[REDACTED-TUNNEL]")
	out = redactMobilePath.ReplaceAllString(out, "/mobile/p/[REDACTED]")
	out = redactSessionQuery.ReplaceAllString(out, `${1}[REDACTED]`)
	if strings.Contains(out, "/mobile/p/") {
		if i := strings.Index(out, "/mobile/p/"); i >= 0 {
			out = out[:i+len("/mobile/p/")] + "[REDACTED]"
		}
	}
	return redactSecrets(out)
}
