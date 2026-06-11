package relay

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	pairRateLimitWindow = time.Minute
	pairRateLimitMax    = 10
)

type pairRateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	now    func() time.Time
	max    int
	window time.Duration
}

func newPairRateLimiter() *pairRateLimiter {
	return &pairRateLimiter{
		hits:   map[string][]time.Time{},
		now:    time.Now,
		max:    pairRateLimitMax,
		window: pairRateLimitWindow,
	}
}

func (l *pairRateLimiter) allow(ip string) bool {
	if l == nil {
		return true
	}
	key := strings.TrimSpace(ip)
	if key == "" {
		key = "unknown"
	}
	now := l.now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()
	stamps := l.hits[key]
	kept := stamps[:0]
	for _, t := range stamps {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

func relayCheckOrigin(r *http.Request) bool {
	if r == nil {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	ou, err := url.Parse(origin)
	if err != nil {
		return false
	}
	ohost := strings.ToLower(ou.Hostname())
	if ohost == "localhost" || ohost == "127.0.0.1" || ohost == "::1" {
		return true
	}
	reqHost := strings.ToLower(r.Host)
	if h, _, err := net.SplitHostPort(reqHost); err == nil {
		reqHost = h
	}
	return ohost == reqHost
}

func pairTokenEqual(stored, provided string) bool {
	a := []byte(stored)
	b := []byte(provided)
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}

func randomSessionID() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return randomID("sess")
	}
	return "sess-" + hex.EncodeToString(buf)
}
