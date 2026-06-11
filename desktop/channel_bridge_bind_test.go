package main

import (
	"testing"
)

func TestClawBridgeBindHostDefault(t *testing.T) {
	if got := clawBridgeBindHost(false); got != clawBridgeBindLAN {
		t.Fatalf("bind host = %q, want %q", got, clawBridgeBindLAN)
	}
	bridge := &clawBridge{mobile: &mobileConnectStore{}}
	if got := bridge.bindHost(); got != clawBridgeBindLAN {
		t.Fatalf("bridge bind host = %q, want %q", got, clawBridgeBindLAN)
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
	addr := clawBridgeListenAddr(clawBridgeBindLAN, defaultClawBridgePort)
	want := "0.0.0.0:8787"
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
	if ip := primaryLANIP(); ip == "" {
		if lan != "" || primary != "" || mode != "none" {
			t.Fatalf("without LAN IP: primary=%q lan=%q mode=%q", primary, lan, mode)
		}
		return
	}
	if lan == "" {
		t.Fatal("expected lan pair URL when LAN IP is available")
	}
	if primary != "" {
		t.Fatalf("primary = %q, want empty until LAN exposure is enabled", primary)
	}
	if mode != "lan_standby" {
		t.Fatalf("mode = %q, want lan_standby", mode)
	}
}

func TestBuildMobilePairURLsTunnelIgnoresLANDisabled(t *testing.T) {
	primary, _, _, mode := buildMobilePairURLs("tok", defaultClawBridgePort, "https://x.trycloudflare.com", "", false, false)
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

func TestLanBindIPDetectsPrimaryLAN(t *testing.T) {
	got := lanBindIP()
	want := primaryLANIP()
	if got != want {
		t.Fatalf("lanBindIP() = %q, want %q", got, want)
	}
}
