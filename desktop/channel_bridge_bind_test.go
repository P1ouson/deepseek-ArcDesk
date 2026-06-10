package main

import (
	"testing"
)

func TestClawBridgeBindHostDefault(t *testing.T) {
	if got := clawBridgeBindHost(false); got != clawBridgeBindLocalhost {
		t.Fatalf("bind host = %q, want %q", got, clawBridgeBindLocalhost)
	}
	bridge := &clawBridge{mobile: &mobileConnectStore{}}
	if got := bridge.bindHost(); got != clawBridgeBindLocalhost {
		t.Fatalf("bridge bind host = %q, want %q", got, clawBridgeBindLocalhost)
	}
}

func TestClawBridgeBindHostAllowLAN(t *testing.T) {
	if got := clawBridgeBindHost(true); got != clawBridgeBindLAN {
		t.Fatalf("bind host = %q, want %q", got, clawBridgeBindLAN)
	}
	store := &mobileConnectStore{}
	store.config.AllowLAN = true
	bridge := &clawBridge{mobile: store}
	if got := bridge.bindHost(); got != clawBridgeBindLAN {
		t.Fatalf("bridge bind host = %q, want %q", got, clawBridgeBindLAN)
	}
}

func TestClawBridgeListenAddrDefault(t *testing.T) {
	addr := clawBridgeListenAddr(clawBridgeBindLocalhost, defaultClawBridgePort)
	want := "127.0.0.1:8787"
	if addr != want {
		t.Fatalf("listen addr = %q, want %q", addr, want)
	}
}

func TestCloudflaredTunnelTargetLocalhost(t *testing.T) {
	got := cloudflaredTunnelTarget(defaultClawBridgePort)
	want := "http://127.0.0.1:8787"
	if got != want {
		t.Fatalf("tunnel target = %q, want %q", got, want)
	}
}

func TestBuildMobilePairURLsWithoutLAN(t *testing.T) {
	primary, lan, _, mode := buildMobilePairURLs("tok", defaultClawBridgePort, "", "", false, false)
	if lan != "" {
		t.Fatalf("lan URL = %q, want empty when LAN disabled", lan)
	}
	if primary != "" {
		t.Fatalf("primary = %q, want empty without tunnel/relay/lan", primary)
	}
	if mode != "none" {
		t.Fatalf("mode = %q, want none", mode)
	}
}

func TestBuildMobilePairURLsTunnelIgnoresLANDisabled(t *testing.T) {
	primary, lan, _, mode := buildMobilePairURLs("tok", defaultClawBridgePort, "https://x.trycloudflare.com", "", false, false)
	if lan != "" {
		t.Fatalf("lan URL = %q, want empty when LAN disabled", lan)
	}
	if primary != "https://x.trycloudflare.com/mobile/p/tok" {
		t.Fatalf("primary = %q", primary)
	}
	if mode != "tunnel" {
		t.Fatalf("mode = %q, want tunnel", mode)
	}
}

func TestBuildMobilePairURLsWithLANUsesLanMode(t *testing.T) {
	// When LAN is enabled and a LAN IP exists, mode should be lan. If this
	// host has no suitable interface the test still verifies we attempted LAN.
	primary, _, _, mode := buildMobilePairURLs("tok", defaultClawBridgePort, "", "", false, true)
	if ip := primaryLANIP(); ip == "" {
		if primary != "" || mode != "none" {
			t.Fatalf("without LAN IP: primary=%q mode=%q", primary, mode)
		}
		return
	}
	if mode != "lan" {
		t.Fatalf("mode = %q, want lan", mode)
	}
	if primary == "" {
		t.Fatal("expected LAN primary URL when LAN IP is available")
	}
}

func TestLanBindIPRespectsAllowLAN(t *testing.T) {
	if lanBindIP(false) != "" {
		t.Fatal("lanBindIP(false) should be empty")
	}
	got := lanBindIP(true)
	want := primaryLANIP()
	if got != want {
		t.Fatalf("lanBindIP(true) = %q, want %q", got, want)
	}
}
