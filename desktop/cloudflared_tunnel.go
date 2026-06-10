package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	cloudflaredDownloadTimeout = 3 * time.Minute
	defaultTunnelIdleTimeout   = 30 * time.Minute
	// cloudflaredPinnedVersion is the only release auto-downloaded by the desktop app.
	// Bump together with cloudflaredReleaseSHA256 when upgrading.
	cloudflaredPinnedVersion = "2025.2.1"
)

var cloudflaredURLPattern = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)

// MobileTunnelStatus reports cloudflared quick-tunnel state.
type MobileTunnelStatus struct {
	Running                bool   `json:"running"`
	URL                    string `json:"url"`
	Err                    string `json:"err,omitempty"`
	PairedCount            int    `json:"pairedCount"`
	AllowLAN               bool   `json:"allowLAN"`
	LocalTarget            string `json:"localTarget"`
	TunnelIdleAutoShutdown bool   `json:"tunnelIdleAutoShutdown"`
	TunnelIdleTimeoutMin   int    `json:"tunnelIdleTimeoutMin"`
}

type cloudflaredTunnel struct {
	mu           sync.Mutex
	cmd          *exec.Cmd
	cancel       context.CancelFunc
	idleCancel   context.CancelFunc
	url          string
	errMsg       string
	lastActivity time.Time
}

func (b *clawBridge) tunnelStatus() MobileTunnelStatus {
	if b == nil {
		return MobileTunnelStatus{}
	}
	if b.tunnel == nil {
		return b.enrichedTunnelStatus(MobileTunnelStatus{})
	}
	b.tunnel.mu.Lock()
	st := MobileTunnelStatus{
		Running: b.tunnel.cmd != nil,
		URL:     b.tunnel.url,
		Err:     b.tunnel.errMsg,
	}
	b.tunnel.mu.Unlock()
	return b.enrichedTunnelStatus(st)
}

func (b *clawBridge) enrichedTunnelStatus(st MobileTunnelStatus) MobileTunnelStatus {
	if b == nil {
		return st
	}
	st.LocalTarget = cloudflaredTunnelTarget(b.port)
	st.TunnelIdleAutoShutdown = b.tunnelIdleAutoShutdownEnabled()
	st.TunnelIdleTimeoutMin = b.tunnelIdleTimeoutMinutes()
	if b.mobile != nil {
		st.PairedCount = b.mobile.pairedCount()
		st.AllowLAN = b.mobile.getConfig().AllowLAN
	}
	return st
}

func (b *clawBridge) tunnelPublicURL() string {
	if b == nil || b.tunnel == nil {
		return ""
	}
	b.tunnel.mu.Lock()
	defer b.tunnel.mu.Unlock()
	return strings.TrimSpace(b.tunnel.url)
}

func (a *App) GetMobileTunnelStatus() MobileTunnelStatus {
	if a == nil || a.clawBridge == nil {
		return MobileTunnelStatus{}
	}
	return a.clawBridge.tunnelStatus()
}

func (a *App) StartMobileTunnel() MobileTunnelStatus {
	if a == nil || a.clawBridge == nil {
		return MobileTunnelStatus{Err: "mobile connect is not ready"}
	}
	if !a.confirmStartMobileTunnel() {
		st := a.clawBridge.tunnelStatus()
		st.Err = "tunnel start cancelled"
		return st
	}
	return a.clawBridge.startCloudflaredTunnel()
}

func (a *App) StopMobileTunnel() MobileTunnelStatus {
	if a == nil || a.clawBridge == nil {
		return MobileTunnelStatus{}
	}
	a.clawBridge.stopCloudflaredTunnel()
	return a.clawBridge.tunnelStatus()
}

func (b *clawBridge) startCloudflaredTunnel() MobileTunnelStatus {
	if b == nil {
		return MobileTunnelStatus{Err: "bridge unavailable"}
	}
	b.mu.Lock()
	if b.tunnel == nil {
		b.tunnel = &cloudflaredTunnel{}
	}
	b.mu.Unlock()

	b.tunnel.mu.Lock()
	if b.tunnel.cmd != nil {
		st := MobileTunnelStatus{Running: true, URL: b.tunnel.url, Err: b.tunnel.errMsg}
		b.tunnel.mu.Unlock()
		return st
	}
	b.tunnel.mu.Unlock()

	bin, err := ensureCloudflaredBinary()
	if err != nil {
		return MobileTunnelStatus{Err: err.Error()}
	}

	ctx, cancel := context.WithCancel(context.Background())
	target := cloudflaredTunnelTarget(b.port)
	cmd := exec.CommandContext(ctx, bin, "tunnel", "--url", target, "--no-autoupdate")
	cmd.Env = os.Environ()
	setCloudflaredCmdAttrs(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return MobileTunnelStatus{Err: err.Error()}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return MobileTunnelStatus{Err: err.Error()}
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return MobileTunnelStatus{Err: err.Error()}
	}

	b.tunnel.mu.Lock()
	b.tunnel.cmd = cmd
	b.tunnel.cancel = cancel
	b.tunnel.url = ""
	b.tunnel.errMsg = ""
	b.tunnel.lastActivity = time.Now()
	idleCtx, idleCancel := context.WithCancel(context.Background())
	b.tunnel.idleCancel = idleCancel
	b.tunnel.mu.Unlock()

	go b.watchCloudflaredOutput(stdout)
	go b.watchCloudflaredOutput(stderr)
	go b.waitCloudflaredExit(cmd)
	go b.watchTunnelIdle(idleCtx)

	slog.Info("cloudflared tunnel starting", "target", target)
	return MobileTunnelStatus{Running: true}
}

func (b *clawBridge) stopCloudflaredTunnel() {
	if b == nil || b.tunnel == nil {
		return
	}
	b.tunnel.mu.Lock()
	cancel := b.tunnel.cancel
	idleCancel := b.tunnel.idleCancel
	cmd := b.tunnel.cmd
	b.tunnel.cmd = nil
	b.tunnel.cancel = nil
	b.tunnel.idleCancel = nil
	b.tunnel.url = ""
	b.tunnel.errMsg = ""
	b.tunnel.mu.Unlock()
	if idleCancel != nil {
		idleCancel()
	}
	if cancel != nil {
		cancel()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (b *clawBridge) touchTunnelActivity() {
	if b == nil || b.tunnel == nil {
		return
	}
	b.tunnel.mu.Lock()
	b.tunnel.lastActivity = time.Now()
	b.tunnel.mu.Unlock()
}

func (b *clawBridge) tunnelIdleAutoShutdownEnabled() bool {
	if b == nil || b.mobile == nil {
		return true
	}
	return !b.mobile.getConfig().TunnelDisableIdleShutdown
}

func (b *clawBridge) tunnelIdleTimeout() time.Duration {
	if b == nil || b.mobile == nil {
		return defaultTunnelIdleTimeout
	}
	minutes := b.mobile.getConfig().TunnelIdleTimeoutMinutes
	if minutes <= 0 {
		minutes = int(defaultTunnelIdleTimeout / time.Minute)
	}
	return time.Duration(minutes) * time.Minute
}

func (b *clawBridge) tunnelIdleTimeoutMinutes() int {
	m := int(b.tunnelIdleTimeout() / time.Minute)
	if m <= 0 {
		return int(defaultTunnelIdleTimeout / time.Minute)
	}
	return m
}

func (b *clawBridge) watchTunnelIdle(ctx context.Context) {
	if b == nil || b.tunnel == nil {
		return
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !b.tunnelIdleAutoShutdownEnabled() {
				continue
			}
			b.tunnel.mu.Lock()
			cmd := b.tunnel.cmd
			last := b.tunnel.lastActivity
			b.tunnel.mu.Unlock()
			if cmd == nil {
				return
			}
			if time.Since(last) >= b.tunnelIdleTimeout() {
				slog.Info("cloudflared tunnel stopped after idle timeout")
				b.stopCloudflaredTunnel()
				return
			}
		}
	}
}

func (b *clawBridge) watchCloudflaredOutput(r io.Reader) {
	if b == nil || b.tunnel == nil || r == nil {
		return
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if url := extractCloudflaredURL(line); url != "" {
			b.tunnel.mu.Lock()
			b.tunnel.url = url
			b.tunnel.errMsg = ""
			b.tunnel.mu.Unlock()
			slog.Info("cloudflared tunnel ready", "url", redactURLQuery(url))
		}
	}
}

func (b *clawBridge) waitCloudflaredExit(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	err := cmd.Wait()
	b.tunnel.mu.Lock()
	defer b.tunnel.mu.Unlock()
	if b.tunnel.cmd != cmd {
		return
	}
	b.tunnel.cmd = nil
	b.tunnel.cancel = nil
	if err != nil && b.tunnel.errMsg == "" {
		b.tunnel.errMsg = err.Error()
	}
}

func extractCloudflaredURL(line string) string {
	match := cloudflaredURLPattern.FindString(line)
	return strings.TrimSpace(match)
}

func ensureCloudflaredBinary() (string, error) {
	if path, err := exec.LookPath("cloudflared"); err == nil {
		return path, nil
	}
	cached := cloudflaredCachedPath()
	if info, err := os.Stat(cached); err == nil && info.Size() > 0 {
		if err := verifyCloudflaredFile(cached); err == nil {
			return cached, nil
		}
		_ = os.Remove(cached)
	}
	if err := downloadCloudflaredBinary(cached); err != nil {
		return "", err
	}
	return cached, nil
}

func cloudflaredCachedPath() string {
	name := "cloudflared"
	if runtime.GOOS == "windows" {
		name = "cloudflared.exe"
	}
	return filepath.Join(ARCDESKDesktopDataPath("bin"), name)
}

type cloudflaredArtifact struct {
	filename string
	sha256   string
}

// SHA256 digests from https://github.com/cloudflare/cloudflared/releases/tag/2025.2.1
var cloudflaredReleaseSHA256 = map[string]string{
	"cloudflared-windows-amd64.exe": "c5479e3ad7a78ba21b1bc56ed2742df2da74bf28612c34c7a7a8a98edc6682f2",
	"cloudflared-linux-amd64":       "afdfadd1ef552e66bffc35246fe30a9bd578356d2d386de95585ccfc432472b8",
	"cloudflared-linux-arm64":       "6d5c61975668e963921db12faf9af7e34c9aa2ba4a3e5b95457c144e1494bf05",
	"cloudflared-linux-arm":         "85bcdcdb484b213b4ac0b3fdf5a5266907539f61aabf4f9bec6cacc24e32e503",
}

func cloudflaredArtifactForPlatform() (cloudflaredArtifact, error) {
	switch runtime.GOOS {
	case "windows":
		if runtime.GOARCH != "amd64" {
			return cloudflaredArtifact{}, fmt.Errorf("cloudflared auto-download is not supported on windows/%s", runtime.GOARCH)
		}
		return cloudflaredArtifact{filename: "cloudflared-windows-amd64.exe", sha256: cloudflaredReleaseSHA256["cloudflared-windows-amd64.exe"]}, nil
	case "darwin":
		return cloudflaredArtifact{}, fmt.Errorf("请先安装 cloudflared（例如：brew install cloudflared）")
	case "linux":
		switch runtime.GOARCH {
		case "arm64":
			return cloudflaredArtifact{filename: "cloudflared-linux-arm64", sha256: cloudflaredReleaseSHA256["cloudflared-linux-arm64"]}, nil
		case "arm":
			return cloudflaredArtifact{filename: "cloudflared-linux-arm", sha256: cloudflaredReleaseSHA256["cloudflared-linux-arm"]}, nil
		case "amd64", "386":
			return cloudflaredArtifact{filename: "cloudflared-linux-amd64", sha256: cloudflaredReleaseSHA256["cloudflared-linux-amd64"]}, nil
		default:
			return cloudflaredArtifact{}, fmt.Errorf("cloudflared auto-download is not supported on linux/%s", runtime.GOARCH)
		}
	default:
		return cloudflaredArtifact{}, fmt.Errorf("cloudflared auto-download is not supported on %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func cloudflaredDownloadURL(artifact cloudflaredArtifact) string {
	return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/%s", cloudflaredPinnedVersion, artifact.filename)
}

func verifyCloudflaredFile(path string) error {
	artifact, err := cloudflaredArtifactForPlatform()
	if err != nil {
		return err
	}
	return verifyFileSHA256(path, artifact.sha256)
}

func verifyFileSHA256(path, want string) error {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return fmt.Errorf("cloudflared: missing sha256 for platform")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("cloudflared: sha256 mismatch: got %s want %s", got, want)
	}
	return nil
}

func downloadCloudflaredBinary(dest string) error {
	artifact, err := cloudflaredArtifactForPlatform()
	if err != nil {
		return err
	}
	url := cloudflaredDownloadURL(artifact)
	if err := ensureParentDir(dest); err != nil {
		return err
	}
	client := &http.Client{Timeout: cloudflaredDownloadTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download cloudflared: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download cloudflared: HTTP %d", resp.StatusCode)
	}
	tmp := dest + ".download"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(out, io.TeeReader(resp.Body, h)); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, artifact.sha256) {
		_ = os.Remove(tmp)
		return fmt.Errorf("cloudflared: sha256 mismatch: got %s want %s", got, artifact.sha256)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(dest, 0o755)
	}
	return nil
}
