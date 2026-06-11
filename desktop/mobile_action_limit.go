package main

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	sessionRateLimitWindow = time.Minute
	sessionRateLimitMax    = 60
)

// actionRateLimiter limits mutating mobile API calls per client IP.
type actionRateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	now    func() time.Time
	max    int
	window time.Duration
}

func newActionRateLimiter(max int, window time.Duration) *actionRateLimiter {
	return &actionRateLimiter{
		hits:   map[string][]time.Time{},
		now:    time.Now,
		max:    max,
		window: window,
	}
}

func newSessionRateLimiter() *actionRateLimiter {
	return newActionRateLimiter(sessionRateLimitMax, sessionRateLimitWindow)
}

func (l *actionRateLimiter) allow(key string) bool {
	if l == nil {
		return true
	}
	key = strings.TrimSpace(key)
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

func (l *actionRateLimiter) allowRequest(r *http.Request) bool {
	if r == nil {
		return l.allow("")
	}
	ip := clientIP(r)
	session := strings.TrimSpace(r.URL.Query().Get("session"))
	if session != "" {
		return l.allow(ip + "|" + session)
	}
	return l.allow(ip)
}
