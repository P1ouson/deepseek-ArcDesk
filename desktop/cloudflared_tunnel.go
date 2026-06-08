package main

import (
	"bufio"
	"context"
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

const cloudflaredDownloadTimeout = 3 * time.Minute

var cloudflaredURLPattern = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)

// MobileTunnelStatus reports cloudflared quick-tunnel state.
type MobileTunnelStatus struct {
	Running bool   `json:"running"`
	URL     string `json:"url"`
	Err     string `json:"err,omitempty"`
}

type cloudflaredTunnel struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	cancel context.CancelFunc
	url    string
	errMsg string
}

func (b *clawBridge) tunnelStatus() MobileTunnelStatus {
	if b == nil || b.tunnel == nil {
		return MobileTunnelStatus{}
	}
	b.tunnel.mu.Lock()
	defer b.tunnel.mu.Unlock()
	return MobileTunnelStatus{
		Running: b.tunnel.cmd != nil,
		URL:     b.tunnel.url,
		Err:     b.tunnel.errMsg,
	}
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
	target := fmt.Sprintf("http://127.0.0.1:%d", b.port)
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
	b.tunnel.mu.Unlock()

	go b.watchCloudflaredOutput(stdout)
	go b.watchCloudflaredOutput(stderr)
	go b.waitCloudflaredExit(cmd)

	slog.Info("cloudflared tunnel starting", "target", target)
	return MobileTunnelStatus{Running: true}
}

func (b *clawBridge) stopCloudflaredTunnel() {
	if b == nil || b.tunnel == nil {
		return
	}
	b.tunnel.mu.Lock()
	cancel := b.tunnel.cancel
	cmd := b.tunnel.cmd
	b.tunnel.cmd = nil
	b.tunnel.cancel = nil
	b.tunnel.url = ""
	b.tunnel.errMsg = ""
	b.tunnel.mu.Unlock()
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
	for scanner.Scan() {
		line := scanner.Text()
		if url := extractCloudflaredURL(line); url != "" {
			b.tunnel.mu.Lock()
			b.tunnel.url = url
			b.tunnel.errMsg = ""
			b.tunnel.mu.Unlock()
			slog.Info("cloudflared tunnel ready", "url", url)
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
		return cached, nil
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

func cloudflaredDownloadURL() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe", nil
	case "darwin":
		return "", fmt.Errorf("请先安装 cloudflared（例如：brew install cloudflared）")
	case "linux":
		switch runtime.GOARCH {
		case "arm64":
			return "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64", nil
		case "arm":
			return "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm", nil
		default:
			return "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64", nil
		}
	default:
		return "", fmt.Errorf("cloudflared auto-download is not supported on %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func downloadCloudflaredBinary(dest string) error {
	url, err := cloudflaredDownloadURL()
	if err != nil {
		return err
	}
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
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
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
