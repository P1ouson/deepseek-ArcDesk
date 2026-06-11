package main

import (
	"bytes"
	"encoding/json"
	goruntime "runtime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"arcdesk/internal/hook"
)

func TestSaveMobileConnectConfigAllowLANRequiresConfirm(t *testing.T) {
	store := newMobileConnectStore()
	bridge := &clawBridge{mobile: store, port: 18786, pairRL: newPairRateLimiter(), sessionRL: newSessionRateLimiter()}
	app := &App{clawBridge: bridge}

	nativeConfirmHook = func(_ NativeConfirmRequest) (bool, error) {
		return false, nil
	}
	defer func() { nativeConfirmHook = nil }()

	err := app.SaveMobileConnectConfig(MobileConnectConfig{Enabled: true, AllowLAN: true})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancelled, got %v", err)
	}
	if store.getConfig().AllowLAN {
		t.Fatal("AllowLAN must not change when confirm is denied")
	}
}

func TestSaveMobileConnectConfigAllowLANAppliesAfterConfirm(t *testing.T) {
	store := newMobileConnectStore()
	bridge := &clawBridge{mobile: store, port: 18786, pairRL: newPairRateLimiter(), sessionRL: newSessionRateLimiter()}
	app := &App{clawBridge: bridge}

	nativeConfirmHook = func(_ NativeConfirmRequest) (bool, error) {
		return true, nil
	}
	defer func() { nativeConfirmHook = nil }()

	if err := app.SaveMobileConnectConfig(MobileConnectConfig{Enabled: true, AllowLAN: true}); err != nil {
		t.Fatal(err)
	}
	if !store.getConfig().AllowLAN {
		t.Fatal("AllowLAN must be enabled in memory during the session")
	}
	path := ARCDESKDesktopDataPath("mobile-connect.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var persisted MobileConnectConfig
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.AllowLAN {
		t.Fatal("AllowLAN must not be persisted to disk")
	}
}

func TestSaveMobileConnectConfigDisableRevokesSessions(t *testing.T) {
	store := newMobileConnectStore()
	store.mu.Lock()
	store.config.Enabled = true
	store.sessions["s1"] = &mobileSession{ID: "s1", CreatedAt: time.Now().UnixMilli(), LastSeen: time.Now().UnixMilli()}
	store.mu.Unlock()
	bridge := &clawBridge{mobile: store, port: 18787, pairRL: newPairRateLimiter(), sessionRL: newSessionRateLimiter()}
	app := &App{clawBridge: bridge}

	if err := app.SaveMobileConnectConfig(MobileConnectConfig{Enabled: false, AllowLAN: false}); err != nil {
		t.Fatal(err)
	}
	if len(store.listSessions()) != 0 {
		t.Fatal("disabling mobile connect should revoke sessions")
	}
}

func TestPairTokenSingleUse(t *testing.T) {
	isolateDesktopUserDirs(t)
	store := newMobileConnectStore()
	store.mu.Lock()
	store.config.Enabled = true
	token := store.pairToken.Token
	store.mu.Unlock()
	bridge := &clawBridge{mobile: store, app: &App{}, port: defaultClawBridgePort, pairRL: newPairRateLimiter()}

	rec1 := postMobilePair(bridge, token)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first pair status = %d", rec1.Code)
	}
	rec2 := postMobilePair(bridge, token)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("second pair with same token want 401, got %d body=%s", rec2.Code, rec2.Body.String())
	}
}

func TestPairRateLimiterBlocksBurst(t *testing.T) {
	rl := &pairRateLimiter{
		hits:   map[string][]time.Time{},
		now:    time.Now,
		max:    3,
		window: time.Minute,
	}
	ip := "10.0.0.1"
	for i := 0; i < 3; i++ {
		if !rl.allow(ip) {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if rl.allow(ip) {
		t.Fatal("fourth attempt should be rate limited")
	}
}

func TestPairTokenEqualConstantLength(t *testing.T) {
	a := randomToken(8)
	if !pairTokenEqual(a, a) {
		t.Fatal("identical tokens should match")
	}
	if pairTokenEqual(a, a+"x") {
		t.Fatal("different length tokens should not match")
	}
}

func TestMobilePairRateLimitHTTP(t *testing.T) {
	isolateDesktopUserDirs(t)
	store := newMobileConnectStore()
	store.mu.Lock()
	store.config.Enabled = true
	token := store.pairToken.Token
	store.mu.Unlock()
	bridge := &clawBridge{
		mobile: store,
		app:    &App{},
		port:   defaultClawBridgePort,
		pairRL: &pairRateLimiter{hits: map[string][]time.Time{}, now: time.Now, max: 2, window: time.Minute},
	}

	body, _ := json.Marshal(map[string]string{"token": token})
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/mobile/api/pair", bytes.NewReader(body))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		bridge.handleMobilePair(rec, req)
		if rec.Code != http.StatusOK && rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", i+1, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/mobile/api/pair", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	bridge.handleMobilePair(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRedactTunnelURL(t *testing.T) {
	in := "cloudflared tunnel ready https://abc123.trycloudflare.com/path"
	out := redactURLQuery(in)
	if strings.Contains(out, "abc123.trycloudflare.com") {
		t.Fatalf("tunnel host leaked: %q", out)
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if err := verifyFileSHA256(path, want); err != nil {
		t.Fatal(err)
	}
	if err := verifyFileSHA256(path, "deadbeef"); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestTrustFileWrittenPrivate(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("unix file modes")
	}
	home := t.TempDir()
	if err := hook.Trust("/proj", home); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(hook.TrustPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("trust.json mode = %o, want owner-only", info.Mode().Perm())
	}
}

func TestMigrateSensitiveDataFileModes(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("unix file modes")
	}
	isolateDesktopUserDirs(t)
	path := ARCDESKDesktopDataPath("claw-channels.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	migrateSensitiveDataFileModes()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("claw-channels.json mode = %o, want owner-only", info.Mode().Perm())
	}
}

func TestCloudflaredPinnedRelease(t *testing.T) {
	if cloudflaredPinnedVersion == "" {
		t.Fatal("missing pinned cloudflared version")
	}
	for _, name := range []string{
		"cloudflared-windows-amd64.exe",
		"cloudflared-linux-amd64",
		"cloudflared-linux-arm64",
		"cloudflared-linux-arm",
	} {
		if cloudflaredReleaseSHA256[name] == "" {
			t.Fatalf("missing sha256 for %s", name)
		}
	}
	url := cloudflaredDownloadURL(cloudflaredArtifact{filename: "cloudflared-linux-amd64"})
	if !strings.Contains(url, cloudflaredPinnedVersion) {
		t.Fatalf("download url = %q", url)
	}
}
