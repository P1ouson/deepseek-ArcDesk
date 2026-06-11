package main

import (
	"testing"
	"time"
)

func TestStopMobileTunnelRevokesSessionsWithoutLAN(t *testing.T) {
	store := newMobileConnectStore()
	store.mu.Lock()
	store.config.AllowLAN = false
	store.sessions["s1"] = &mobileSession{
		ID:        "s1",
		CreatedAt: time.Now().UnixMilli(),
		LastSeen:  time.Now().UnixMilli(),
	}
	store.mu.Unlock()
	bridge := &clawBridge{
		mobile: store,
		tunnel: &mobileTunnelRunner{url: "https://x.trycloudflare.com"},
	}

	bridge.stopMobileTunnel()

	if len(store.listAllSessions()) != 0 {
		t.Fatal("tunnel stop should revoke sessions when LAN is disabled")
	}
	if store.activeSessionCount() != 0 {
		t.Fatal("active count should be zero after tunnel stop")
	}
}

func TestStopMobileTunnelMarksSessionsInactiveWithLAN(t *testing.T) {
	store := newMobileConnectStore()
	store.mu.Lock()
	store.config.AllowLAN = true
	store.sessions["s1"] = &mobileSession{
		ID:        "s1",
		CreatedAt: time.Now().UnixMilli(),
		LastSeen:  time.Now().UnixMilli(),
	}
	store.mu.Unlock()
	bridge := &clawBridge{
		mobile: store,
		tunnel: &mobileTunnelRunner{url: "https://x.trycloudflare.com"},
	}

	bridge.stopMobileTunnel()

	if len(store.listAllSessions()) != 1 {
		t.Fatal("LAN mode should keep session history after tunnel stop")
	}
	if store.activeSessionCount() != 0 {
		t.Fatal("active count should drop immediately after tunnel stop")
	}
	if len(store.listActiveSessions()) != 0 {
		t.Fatal("active session list should be empty after tunnel stop")
	}
}

func TestListMobileSessionsReturnsActiveOnly(t *testing.T) {
	store := newMobileConnectStore()
	now := time.Now().UnixMilli()
	store.mu.Lock()
	store.sessions["active"] = &mobileSession{ID: "active", CreatedAt: now, LastSeen: now}
	store.sessions["idle"] = &mobileSession{
		ID:        "idle",
		CreatedAt: now,
		LastSeen:  now - mobileSessionActiveWindow.Milliseconds() - 1,
	}
	store.mu.Unlock()

	items := store.listSessions()
	if len(items) != 1 || items[0].ID != "active" {
		t.Fatalf("listSessions = %+v, want only active session", items)
	}
}
