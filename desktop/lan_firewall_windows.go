//go:build windows

package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"arcdesk/internal/proc"
)

const (
	mobileFirewallPortRule = "ArcDesk Mobile Bridge (TCP 8787)"
	mobileFirewallAppRule  = "ArcDesk Mobile Bridge (App)"
)

func ensureMobileLANFirewallRule(port int) {
	if port <= 0 {
		port = defaultClawBridgePort
	}
	deleteFirewallRule(mobileFirewallPortRule)
	deleteFirewallRule(mobileFirewallAppRule)

	portStr := fmt.Sprintf("%d", port)
	addFirewallRule(
		mobileFirewallPortRule,
		"dir=in", "action=allow", "protocol=TCP", "localport="+portStr,
		"remoteip=any", "profile=any", "enable=yes",
	)

	if exe, err := os.Executable(); err == nil {
		exe, _ = filepath.Abs(exe)
		addFirewallRule(
			mobileFirewallAppRule,
			"dir=in", "action=allow", "program="+exe, "enable=yes", "profile=any",
		)
	}
}

func deleteFirewallRule(name string) {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name="+name)
	proc.HideWindowDetached(cmd)
	_, _ = cmd.CombinedOutput()
}

func addFirewallRule(name string, args ...string) {
	params := append([]string{"advfirewall", "firewall", "add", "rule", "name=" + name}, args...)
	cmd := exec.Command("netsh", params...)
	proc.HideWindowDetached(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		slog.Warn("mobile LAN firewall rule not added", "rule", name, "err", err, "out", strings.TrimSpace(string(out)))
	}
}

func ruleExists(name string) bool {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", "name="+name)
	proc.HideWindowDetached(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), name)
}
