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
	bridge := &clawBridge{mobile: store, port: defaultClawBridgePort, pairRL: newPairRateLimiter()}
	app := &App{clawBridge: bridge}

	nativeConfirmHook = func(_ NativeConfirmRequest) (bool, error) { return false, nil }
	defer func() { nativeConfirmHook = nil }()

	err := app.SaveMobileConnectConfig(MobileConnectConfig{Enabled: true, AllowLAN: true})
	if err == nil {
		t.Fatal("expected cancelled error")
	}
	if store.getConfig().AllowLAN {
		t.Fatal("AllowLAN must not persist when confirm is cancelled")
	}
}

func TestSaveMobileConnectConfigAllowLANOffSkipsConfirm(t *testing.T) {
	store := newMobileConnectStore()
	store.mu.Lock()
	store.config.AllowLAN = true
	store.mu.Unlock()
	bridge := &clawBridge{mobile: store, port: defaultClawBridgePort, pairRL: newPairRateLimiter()}
	app := &App{clawBridge: bridge}

	called := false
	nativeConfirmHook = func(_ NativeConfirmRequest) (bool, error) {
		called = true
		return true, nil
	}
	defer func() { nativeConfirmHook = nil }()

	if err := app.SaveMobileConnectConfig(MobileConnectConfig{Enabled: true, AllowLAN: false}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("disabling AllowLAN should not prompt")
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
