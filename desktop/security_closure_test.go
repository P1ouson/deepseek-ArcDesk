package main

import (
	"strings"
	"testing"
	"time"

	"arcdesk/internal/config"
)

func TestDecisionRouteRejectsCrossTab(t *testing.T) {
	s := newDecisionRouteStore()
	s.register("42", "tab-a")
	if tab, ok := s.resolve("tab-b", "42", "tab-b"); ok {
		t.Fatalf("cross-tab approval should be rejected, got tab=%q", tab)
	}
	if tab, ok := s.resolve("", "42", "tab-a"); !ok || tab != "tab-a" {
		t.Fatalf("active tab matching route should succeed, got tab=%q ok=%v", tab, ok)
	}
}

func TestStartMobileTunnelStartsWithoutConfirm(t *testing.T) {
	store := newMobileConnectStore()
	app := &App{clawBridge: &clawBridge{port: defaultClawBridgePort, mobile: store, pairRL: newPairRateLimiter()}}
	called := false
	nativeConfirmHook = func(_ NativeConfirmRequest) (bool, error) {
		called = true
		return false, nil
	}
	defer func() { nativeConfirmHook = nil }()

	// startCloudflaredTunnel may fail without cloudflared binary; we only assert confirm is not shown.
	_ = app.StartMobileTunnel()
	if called {
		t.Fatal("StartMobileTunnel should not show native confirm (Connect UI already warns the user)")
	}
}

func TestTunnelStatusIncludesAuthState(t *testing.T) {
	store := newMobileConnectStore()
	store.sessions["sess-1"] = &mobileSession{
		ID:        "sess-1",
		CreatedAt: time.Now().UnixMilli(),
		LastSeen:  time.Now().UnixMilli(),
	}
	store.config.AllowLAN = true
	b := &clawBridge{port: defaultClawBridgePort, mobile: store, tunnel: &mobileTunnelRunner{}}

	st := b.tunnelStatus()
	if st.PairedCount != 1 {
		t.Fatalf("pairedCount = %d, want 1", st.PairedCount)
	}
	if !st.AllowLAN {
		t.Fatal("allowLAN should be true")
	}
	if st.LocalTarget != cloudflaredTunnelTarget(defaultClawBridgePort) {
		t.Fatalf("localTarget = %q", st.LocalTarget)
	}
	if !st.TunnelIdleAutoShutdown {
		t.Fatal("idle auto-shutdown should default enabled")
	}
}

func TestMobileSessionIdleExpiry(t *testing.T) {
	now := time.Now().UnixMilli()
	sess := &mobileSession{
		CreatedAt: now,
		LastSeen:  now - mobileSessionIdleTTL.Milliseconds() - 1,
	}
	if mobileSessionStillValid(sess, now) {
		t.Fatal("idle-expired session should be invalid")
	}
}

func TestMobileSessionMaxAgeExpiry(t *testing.T) {
	now := time.Now().UnixMilli()
	sess := &mobileSession{
		CreatedAt: now - mobileSessionMaxAge.Milliseconds() - 1,
		LastSeen:  now,
	}
	if mobileSessionStillValid(sess, now) {
		t.Fatal("max-age expired session should be invalid")
	}
}

func TestListPromotableProviderKeys(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("DEEPSEEK_API_KEY", "sk-test")
	keys := listPromotableProviderKeys(config.Default())
	found := false
	for _, k := range keys {
		if k == "DEEPSEEK_API_KEY" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected DEEPSEEK_API_KEY promotable, got %v", keys)
	}
}

func TestRedactSecrets(t *testing.T) {
	in := "Authorization: Bearer abcdefghijklmnop token=sk-1234567890abcdef"
	out := redactSecrets(in)
	if out == in {
		t.Fatal("expected redaction")
	}
	if strings.Contains(out, "abcdefghijklmnop") || strings.Contains(out, "sk-1234567890abcdef") {
		t.Fatalf("secret material leaked: %q", out)
	}
}

func TestResolveDecisionTabRejectsMismatch(t *testing.T) {
	app := &App{
		decisionRoutes: newDecisionRouteStore(),
		tabs:           map[string]*WorkspaceTab{},
		activeTabID:    "tab-b",
	}
	app.decisionRoutes.register("9", "tab-a")
	if tab, ok := app.resolveDecisionTab("tab-b", "9"); ok {
		t.Fatalf("expected reject, got tab=%q", tab)
	}
}
