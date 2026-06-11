package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClawBridgeBindHostAlwaysLAN(t *testing.T) {
	if got := clawBridgeBindHost(false); got != clawBridgeBindLAN {
		t.Fatalf("bind host = %q, want %q", got, clawBridgeBindLAN)
	}
	if got := clawBridgeBindHost(true); got != clawBridgeBindLAN {
		t.Fatalf("bind host = %q, want %q", got, clawBridgeBindLAN)
	}
	bridge := &clawBridge{mobile: &mobileConnectStore{}}
	if got := bridge.bindHost(); got != clawBridgeBindLAN {
		t.Fatalf("bridge bind host = %q, want %q", got, clawBridgeBindLAN)
	}
}

func TestMobileAccessAllowedLoopbackWhenLANDisabled(t *testing.T) {
	store := newMobileConnectStore()
	store.config.AllowLAN = false
	bridge := &clawBridge{mobile: store}

	req := httptest.NewRequest(http.MethodGet, "/mobile/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	if !bridge.mobileAccessAllowed(req) {
		t.Fatal("loopback should be allowed when LAN is disabled")
	}
}

func TestMobileAccessDeniedRemoteWhenLANDisabled(t *testing.T) {
	store := newMobileConnectStore()
	store.config.AllowLAN = false
	bridge := &clawBridge{mobile: store}

	req := httptest.NewRequest(http.MethodGet, "/mobile/health", nil)
	req.RemoteAddr = "192.168.0.50:12345"
	if bridge.mobileAccessAllowed(req) {
		t.Fatal("remote LAN client should be blocked when LAN is disabled")
	}
}

func TestMobileAccessAllowedRemoteWhenLANEnabled(t *testing.T) {
	store := newMobileConnectStore()
	store.config.AllowLAN = true
	bridge := &clawBridge{mobile: store}

	req := httptest.NewRequest(http.MethodGet, "/mobile/health", nil)
	req.RemoteAddr = "192.168.0.50:12345"
	if !bridge.mobileAccessAllowed(req) {
		t.Fatal("remote LAN client should be allowed when LAN is enabled")
	}
}

func TestMobileAccessMiddlewareBlocksRemote(t *testing.T) {
	store := newMobileConnectStore()
	store.config.AllowLAN = false
	bridge := &clawBridge{mobile: store}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mobile/health", nil)
	req.RemoteAddr = "192.168.0.50:12345"
	bridge.withMobileAccess(bridge.handleMobileHealth)(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
