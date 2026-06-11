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
	defaultTunnelIdleTimeout   = 10 * time.Minute
	tunnelStartupTimeout       = 90 * time.Second
	// cloudflaredPinnedVersion is the only release auto-downloaded by the desktop app.
	// Bump together with cloudflaredReleaseSHA256 when upgrading.
	cloudflaredPinnedVersion = "2025.2.1"
)

var cloudflaredURLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`),
	regexp.MustCompile(`https://[a-zA-Z0-9-]+\.cfargotunnel\.com`),
}

// MobileTunnelStatus reports Cloudflare quick-tunnel state.
type MobileTunnelStatus struct {
	Running                bool   `json:"running"`
	URL                    string `json:"url"`
	Err                    string `json:"err,omitempty"`
	Phase                  string `json:"phase,omitempty"`
	DownloadProgress       int    `json:"downloadProgress,omitempty"`
	PairedCount            int    `json:"pairedCount"`
	ActiveCount            int    `json:"activeCount"`
	AllowLAN               bool   `json:"allowLAN"`
	LocalTarget            string `json:"localTarget"`
	TunnelIdleAutoShutdown bool   `json:"tunnelIdleAutoShutdown"`
	TunnelIdleTimeoutMin   int    `json:"tunnelIdleTimeoutMin"`
}

type mobileTunnelRunner struct {
	mu               sync.Mutex
	phase            string
	cmd              *exec.Cmd
	cancel           context.CancelFunc
	idleCancel       context.CancelFunc
	url              string
	errMsg           string
	downloadProgress int
	lastActivity     time.Time
}

func (t *mobileTunnelRunner) active() bool {
	if t == nil {
		return false
	}
	if t.cmd != nil {
		return true
	}
	switch t.phase {
	case "downloading", "starting":
		return true
	default:
		return false
	}
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
		Running:          b.tunnel.active(),
		URL:              b.tunnel.url,
		Err:              b.tunnel.errMsg,
		Phase:            b.tunnel.phase,
		DownloadProgress: b.tunnel.downloadProgress,
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
		st.ActiveCount = b.mobile.activeSessionCount()
		st.AllowLAN = b.mobile.getConfig().AllowLAN
	}
	if st.Running && st.URL != "" && st.Phase == "" {
		st.Phase = "ready"
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
	if a == nil {
		return MobileTunnelStatus{Err: "mobile connect is not ready"}
	}
	if !a.confirmStartMobileTunnel() {
		return MobileTunnelStatus{Err: "cancelled"}
	}
	if err := a.ensureClawBridge(); err != nil {
		return MobileTunnelStatus{Err: err.Error()}
	}
	if a.clawBridge == nil {
		return MobileTunnelStatus{Err: "mobile connect is not ready"}
	}
	return a.clawBridge.startCloudflaredTunnel()
}

func (a *App) StopMobileTunnel() MobileTunnelStatus {
	if a == nil || a.clawBridge == nil {
		return MobileTunnelStatus{}
	}
	a.clawBridge.stopMobileTunnel()
	return a.clawBridge.tunnelStatus()
}

func (b *clawBridge) startCloudflaredTunnel() MobileTunnelStatus {
	if b == nil {
		return MobileTunnelStatus{Err: "bridge unavailable"}
	}
	b.mu.Lock()
	if b.tunnel == nil {
		b.tunnel = &mobileTunnelRunner{}
	}
	b.mu.Unlock()

	b.tunnel.mu.Lock()
	if b.tunnel.active() {
		st := MobileTunnelStatus{
			Running:          true,
			URL:              b.tunnel.url,
			Err:              b.tunnel.errMsg,
			Phase:            b.tunnel.phase,
			DownloadProgress: b.tunnel.downloadProgress,
		}
		b.tunnel.mu.Unlock()
		return b.enrichedTunnelStatus(st)
	}
	b.tunnel.phase = "downloading"
	b.tunnel.url = ""
	b.tunnel.errMsg = ""
	b.tunnel.downloadProgress = -1
	b.tunnel.mu.Unlock()

	go b.startCloudflaredTunnelAsync()
	return b.enrichedTunnelStatus(MobileTunnelStatus{Running: true, Phase: "downloading", DownloadProgress: -1})
}

func (b *clawBridge) startCloudflaredTunnelAsync() {
	progress := func(pct int) {
		if b == nil || b.tunnel == nil {
			return
		}
		b.tunnel.mu.Lock()
		b.tunnel.downloadProgress = pct
		b.tunnel.mu.Unlock()
	}
	bin, err := ensureCloudflaredBinary(progress)
	if err != nil {
		b.tunnel.mu.Lock()
		b.tunnel.phase = "error"
		b.tunnel.errMsg = err.Error()
		b.tunnel.mu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	target := cloudflaredTunnelTarget(b.port)
	cmd := exec.CommandContext(ctx, bin, "tunnel", "--url", target, "--no-autoupdate")
	cmd.Env = os.Environ()
	setCloudflaredCmdAttrs(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		b.setTunnelError(err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		b.setTunnelError(err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		cancel()
		b.setTunnelError(err.Error())
		return
	}

	b.tunnel.mu.Lock()
	b.tunnel.cmd = cmd
	b.tunnel.cancel = cancel
	b.tunnel.phase = "starting"
	b.tunnel.downloadProgress = 0
	b.tunnel.lastActivity = time.Now()
	idleCtx, idleCancel := context.WithCancel(context.Background())
	b.tunnel.idleCancel = idleCancel
	b.tunnel.mu.Unlock()

	go b.watchCloudflaredOutput(stdout)
	go b.watchCloudflaredOutput(stderr)
	go b.waitTunnelProcessExit(cmd)
	go b.watchTunnelIdle(idleCtx)
	go b.watchTunnelStartupTimeout(cmd)

	slog.Info("cloudflared tunnel starting", "target", target)
}

func (b *clawBridge) setTunnelError(msg string) {
	if b == nil || b.tunnel == nil {
		return
	}
	b.tunnel.mu.Lock()
	b.tunnel.phase = "error"
	b.tunnel.errMsg = msg
	b.tunnel.mu.Unlock()
}

func (b *clawBridge) stopMobileTunnel() {
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
	b.tunnel.phase = ""
	b.tunnel.downloadProgress = 0
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
	b.disconnectRemoteSessions()
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
				slog.Info("mobile tunnel stopped after idle timeout")
				b.stopMobileTunnel()
				return
			}
		}
	}
}

func (b *clawBridge) watchTunnelStartupTimeout(cmd *exec.Cmd) {
	time.Sleep(tunnelStartupTimeout)
	if b == nil || b.tunnel == nil {
		return
	}
	b.tunnel.mu.Lock()
	if b.tunnel.cmd != cmd || b.tunnel.url != "" {
		b.tunnel.mu.Unlock()
		return
	}
	b.tunnel.phase = "error"
	b.tunnel.errMsg = "Cloudflare 穿透超时（国内网络可能无法连接 GitHub/Cloudflare）。请稍后重试。"
	cancel := b.tunnel.cancel
	idleCancel := b.tunnel.idleCancel
	b.tunnel.cmd = nil
	b.tunnel.cancel = nil
	b.tunnel.idleCancel = nil
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

func (b *clawBridge) watchCloudflaredOutput(r io.Reader) {
	if b == nil || b.tunnel == nil || r == nil {
		return
	}
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if u := extractCloudflaredURL(line); u != "" {
			b.tunnel.mu.Lock()
			b.tunnel.url = u
			b.tunnel.errMsg = ""
			b.tunnel.phase = "ready"
			b.tunnel.mu.Unlock()
			slog.Info("cloudflared tunnel ready", "url", redactURLQuery(u))
		}
	}
}

func (b *clawBridge) waitTunnelProcessExit(cmd *exec.Cmd) {
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
	if err != nil && b.tunnel.errMsg == "" && b.tunnel.url == "" {
		b.tunnel.phase = "error"
		b.tunnel.errMsg = err.Error()
	}
}

func extractCloudflaredURL(line string) string {
	for _, pattern := range cloudflaredURLPatterns {
		if match := pattern.FindString(line); match != "" {
			return strings.TrimSpace(match)
		}
	}
	return ""
}

func ensureCloudflaredBinary(onProgress func(int)) (string, error) {
	if path, err := exec.LookPath("cloudflared"); err == nil {
		return path, nil
	}
	cached := cloudflaredCachedPath()
	if info, err := os.Stat(cached); err == nil && info.Size() > 0 {
		if err := verifyCloudflaredFile(cached); err == nil {
			if onProgress != nil {
				onProgress(100)
			}
			return cached, nil
		}
		_ = os.Remove(cached)
	}
	if err := downloadCloudflaredBinary(cached, onProgress); err != nil {
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

func downloadCloudflaredBinary(dest string, onProgress func(int)) error {
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
	total := resp.ContentLength
	if onProgress != nil {
		if total > 0 {
			onProgress(0)
		} else {
			onProgress(-1)
		}
	}
	var downloaded int64
	src := io.TeeReader(resp.Body, h)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				out.Close()
				_ = os.Remove(tmp)
				return err
			}
			downloaded += int64(n)
			if onProgress != nil && total > 0 {
				pct := int(downloaded * 100 / total)
				if pct > 100 {
					pct = 100
				}
				onProgress(pct)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			_ = os.Remove(tmp)
			return readErr
		}
	}
	if onProgress != nil && total > 0 {
		onProgress(100)
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
