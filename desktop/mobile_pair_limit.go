package main

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	pairRateLimitWindow = time.Minute
	pairRateLimitMax    = 10
)

// pairRateLimiter limits POST /mobile/api/pair attempts per client IP.
type pairRateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	now     func() time.Time
	max     int
	window  time.Duration
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
