package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// MobileConnectDiagnostics is a copy-paste report for troubleshooting phone pairing.
type MobileConnectDiagnostics struct {
	Report      string `json:"report"`
	BridgeReady bool   `json:"bridgeReady"`
	AllowLAN    bool   `json:"allowLAN"`
	BindAddress string `json:"bindAddress"`
	LanIP       string `json:"lanIp"`
	Port        int    `json:"port"`
	ConnectMode string `json:"connectMode"`
	PairURL     string `json:"pairUrl"`
	LocalHealth string `json:"localHealth"`
	LanHealth   string `json:"lanHealth"`
}

func probeHTTPHealth(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "skipped"
	}
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(baseURL + "/mobile/health")
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return "ok"
}

func tcpReachable(host string, port int) string {
	addr := clawBridgeListenAddr(host, port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return err.Error()
	}
	_ = conn.Close()
	return "ok"
}

func (a *App) GetMobileConnectDiagnostics() MobileConnectDiagnostics {
	port := defaultClawBridgePort
	lanIP := primaryLANIP()
	out := MobileConnectDiagnostics{
		Port:  port,
		LanIP: lanIP,
	}

	if a == nil || a.clawBridge == nil || a.clawBridge.mobile == nil {
		out.LocalHealth = "bridge not started"
		out.LanHealth = "bridge not started"
		out.Report = formatMobileDiagnostics(out, nil)
		return out
	}

	cfg := a.clawBridge.mobile.getConfig()
	pairing := a.clawBridge.mobilePairingInfo()
	bindHost := a.clawBridge.bindHost()

	out.BridgeReady = true
	out.AllowLAN = cfg.AllowLAN
	out.BindAddress = clawBridgeListenAddr(bindHost, port)
	out.ConnectMode = pairing.ConnectMode
	out.PairURL = pairing.PairURL
	if out.PairURL == "" {
		out.PairURL = pairing.LanPairURL
	}
	out.LocalHealth = probeHTTPHealth(fmt.Sprintf("http://127.0.0.1:%d", port))
	if cfg.AllowLAN && lanIP != "" {
		out.LanHealth = probeHTTPHealth(fmt.Sprintf("http://%s:%d", lanIP, port))
	} else if !cfg.AllowLAN {
		out.LanHealth = "LAN pairing disabled"
	} else {
		out.LanHealth = "no LAN IP detected"
	}

	out.Report = formatMobileDiagnostics(out, &cfg)
	return out
}

func formatMobileDiagnostics(d MobileConnectDiagnostics, _ *MobileConnectConfig) string {
	var b strings.Builder
	b.WriteString("=== ArcDesk 手机连接诊断 ===\n")
	b.WriteString(fmt.Sprintf("桥接就绪: %v\n", d.BridgeReady))
	b.WriteString(fmt.Sprintf("局域网配对: %v\n", d.AllowLAN))
	b.WriteString(fmt.Sprintf("监听地址: %s\n", d.BindAddress))
	b.WriteString(fmt.Sprintf("局域网 IP: %s\n", emptyDash(d.LanIP)))
	b.WriteString(fmt.Sprintf("连接模式: %s\n", emptyDash(d.ConnectMode)))
	b.WriteString(fmt.Sprintf("配对 URL: %s\n", emptyDash(d.PairURL)))
	b.WriteString(fmt.Sprintf("本机健康检查 (127.0.0.1:%d/mobile/health): %s\n", d.Port, d.LocalHealth))
	b.WriteString(fmt.Sprintf("局域网健康检查: %s\n", d.LanHealth))
	if d.LanIP != "" && d.AllowLAN {
		b.WriteString(fmt.Sprintf("TCP %s:%d: %s\n", d.LanIP, d.Port, tcpReachable(d.LanIP, d.Port)))
	}
	b.WriteString(fmt.Sprintf("本机 TCP 127.0.0.1:%d: %s\n", d.Port, tcpReachable("127.0.0.1", d.Port)))
	b.WriteString("\n注意：电脑浏览器能打开局域网地址，不代表手机一定能连上。\n")
	b.WriteString("若手机 Safari 显示「服务器已停止响应」，常见原因：\n")
	b.WriteString("  1) Windows 防火墙拦截其他设备入站（已尝试自动添加规则，若仍失败请在「Windows 安全中心 → 防火墙」允许 arcdesk-desktop.exe）\n")
	b.WriteString("  2) 路由器开启了 AP/客户端隔离（同 Wi‑Fi 下设备互不可见）→ 请改用「一键穿透」\n")
	b.WriteString("  3) 手机用了蜂窝数据而非同一 Wi‑Fi\n")
	b.WriteString("\n手机端请把浏览器里显示的完整错误原文贴给开发者。\n")
	return b.String()
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
