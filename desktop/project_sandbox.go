package main

import (
	"arcdesk/internal/config"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const projectSandboxFileName = "project-sandbox.json"

// ProjectSandboxProfile is the per-project sandbox contract the desktop UI
// configures before high-trust modes (YOLO) or optional hardening elsewhere.
// Agent bash/file enforcement still reads arcdesk.toml [sandbox]; this file
// marks explicit user consent and drives Web preview URL policy.
type ProjectSandboxProfile struct {
	Configured    bool     `json:"configured"`
	ConfiguredAt  int64    `json:"configuredAt,omitempty"`
	WorkspaceRoot string   `json:"workspaceRoot"`
	Bash          string   `json:"bash"` // enforce | off
	Network       bool     `json:"network"`
	AllowWrite    []string `json:"allowWrite"`
	PreviewHosts  []string `json:"previewHosts"`
	PreviewPorts  []int    `json:"previewPorts"`
	PreviewStrict bool     `json:"previewStrict"` // when true, only whitelist URLs may load (not just in YOLO)
}

// ProjectSandboxStatus is the frontend read model.
type ProjectSandboxStatus struct {
	Configured    bool     `json:"configured"`
	WorkspaceRoot string   `json:"workspaceRoot"`
	Bash          string   `json:"bash"`
	Network       bool     `json:"network"`
	AllowWrite    []string `json:"allowWrite"`
	PreviewHosts  []string `json:"previewHosts"`
	PreviewPorts  []int    `json:"previewPorts"`
	PreviewStrict bool     `json:"previewStrict"`
	YoloRequired  bool     `json:"yoloRequired"`
}

// ConfigureProjectSandboxInput is written from the setup wizard.
type ConfigureProjectSandboxInput struct {
	Bash          string   `json:"bash"`
	Network       bool     `json:"network"`
	AllowWrite    []string `json:"allowWrite"`
	PreviewHosts  []string `json:"previewHosts"`
	PreviewPorts  []int    `json:"previewPorts"`
	PreviewStrict bool     `json:"previewStrict"`
}

// PreviewURLValidation is returned to the Web preview panel.
type PreviewURLValidation struct {
	Decision string `json:"decision"` // allow | confirm | blocked
	URL      string `json:"url"`
	Reason   string `json:"reason,omitempty"`
	Strict   bool   `json:"strict"` // true when YOLO project sandbox is active
}

func projectSandboxPath(workspaceRoot string) string {
	return projectSandboxPrimaryPath(workspaceRoot)
}

func projectSandboxLegacyPath(workspaceRoot string) string {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" || root == "." {
		return filepath.Join(config.LegacyProjectMetaDir, projectSandboxFileName)
	}
	return filepath.Join(root, config.LegacyProjectMetaDir, projectSandboxFileName)
}

func projectSandboxPrimaryPath(workspaceRoot string) string {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" || root == "." {
		return filepath.Join(config.ProjectMetaDir, projectSandboxFileName)
	}
	return filepath.Join(root, config.ProjectMetaDir, projectSandboxFileName)
}

func defaultProjectSandboxProfile(workspaceRoot string) ProjectSandboxProfile {
	hosts := []string{"localhost", "127.0.0.1", "[::1]"}
	ports := []int{5173, 3000, 8080, 4173}
	if root := strings.TrimSpace(workspaceRoot); root != "" && root != "." {
		if abs, err := filepath.Abs(root); err == nil {
			hosts = append(hosts, abs)
		}
	}
	return ProjectSandboxProfile{
		WorkspaceRoot: workspaceRoot,
		Bash:          "enforce",
		Network:       true,
		PreviewHosts:  hosts,
		PreviewPorts:  ports,
	}
}

func loadProjectSandboxProfile(workspaceRoot string) (ProjectSandboxProfile, error) {
	path := projectSandboxPath(workspaceRoot)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			legacyPath := projectSandboxLegacyPath(workspaceRoot)
			if legacyPath != path {
				if raw2, err2 := os.ReadFile(legacyPath); err2 == nil {
					raw = raw2
				} else if os.IsNotExist(err2) {
					return defaultProjectSandboxProfile(workspaceRoot), nil
				} else {
					return ProjectSandboxProfile{}, err2
				}
			} else {
				return defaultProjectSandboxProfile(workspaceRoot), nil
			}
		} else {
			return ProjectSandboxProfile{}, err
		}
	}
	var p ProjectSandboxProfile
	if err := json.Unmarshal(raw, &p); err != nil {
		return ProjectSandboxProfile{}, err
	}
	if p.Bash == "" {
		p.Bash = "enforce"
	}
	if len(p.PreviewHosts) == 0 {
		def := defaultProjectSandboxProfile(workspaceRoot)
		p.PreviewHosts = def.PreviewHosts
	}
	if len(p.PreviewPorts) == 0 {
		def := defaultProjectSandboxProfile(workspaceRoot)
		p.PreviewPorts = def.PreviewPorts
	}
	return p, nil
}

func saveProjectSandboxProfile(workspaceRoot string, p ProjectSandboxProfile) error {
	path := projectSandboxPath(workspaceRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func projectSandboxStatus(workspaceRoot string, yoloActive bool) ProjectSandboxStatus {
	p, _ := loadProjectSandboxProfile(workspaceRoot)
	return ProjectSandboxStatus{
		Configured:    p.Configured,
		WorkspaceRoot: p.WorkspaceRoot,
		Bash:          p.Bash,
		Network:       p.Network,
		AllowWrite:    append([]string(nil), p.AllowWrite...),
		PreviewHosts:  append([]string(nil), p.PreviewHosts...),
		PreviewPorts:  append([]int(nil), p.PreviewPorts...),
		PreviewStrict: p.PreviewStrict,
		YoloRequired:  yoloActive && !p.Configured,
	}
}

func validatePreviewURL(raw string, profile ProjectSandboxProfile, strict bool) PreviewURLValidation {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return PreviewURLValidation{Decision: "blocked", Reason: "invalid", Strict: strict}
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"javascript:", "data:", "file:", "vbscript:", "about:", "blob:"} {
		if strings.HasPrefix(lower, prefix) {
			return PreviewURLValidation{Decision: "blocked", URL: trimmed, Reason: "unsafe-scheme", Strict: strict}
		}
	}

	parsed, err := url.Parse(trimmed)
	if !strings.Contains(trimmed, "://") {
		parsed, err = url.Parse("http://" + trimmed)
	}
	if err != nil {
		return PreviewURLValidation{Decision: "blocked", URL: trimmed, Reason: "invalid", Strict: strict}
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "http", "https":
	default:
		return PreviewURLValidation{Decision: "blocked", URL: parsed.String(), Reason: "unsafe-scheme", Strict: strict}
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return PreviewURLValidation{Decision: "blocked", URL: parsed.String(), Reason: "invalid", Strict: strict}
	}

	if strict {
		if previewAllowedByProfile(parsed, profile) {
			return PreviewURLValidation{Decision: "allow", URL: parsed.String(), Strict: true}
		}
		return PreviewURLValidation{Decision: "blocked", URL: parsed.String(), Reason: "not-in-profile", Strict: true}
	}

	if isLocalPreviewHost(host) {
		return PreviewURLValidation{Decision: "allow", URL: parsed.String(), Strict: false}
	}
	return PreviewURLValidation{Decision: "confirm", URL: parsed.String(), Reason: "external", Strict: false}
}

func isLocalPreviewHost(host string) bool {
	h := strings.ToLower(strings.Trim(host, "[]"))
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

func previewAllowedByProfile(u *url.URL, profile ProjectSandboxProfile) bool {
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	portNum, _ := strconv.Atoi(port)

	for _, allowedHost := range profile.PreviewHosts {
		ah := strings.ToLower(strings.Trim(allowedHost, "[]"))
		if ah == host {
			return portAllowed(portNum, profile.PreviewPorts)
		}
	}
	return false
}

func portAllowed(port int, allowed []int) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, p := range allowed {
		if p == port {
			return true
		}
	}
	return false
}

func normalizePreviewPorts(ports []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(ports))
	for _, p := range ports {
		if p < 1 || p > 65535 {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func normalizePreviewHosts(hosts []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if strings.Contains(h, "*") {
			continue
		}
		if strings.HasPrefix(h, ".") {
			continue
		}
		if net.ParseIP(h) != nil {
			h = strings.ToLower(h)
		}
		key := strings.ToLower(h)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, h)
	}
	return out
}

func mergeSandboxInput(workspaceRoot string, in ConfigureProjectSandboxInput) (ProjectSandboxProfile, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		root = "."
	}
	bash := strings.TrimSpace(in.Bash)
	if bash != "off" {
		bash = "enforce"
	}
	profile := ProjectSandboxProfile{
		Configured:    true,
		ConfiguredAt:  time.Now().Unix(),
		WorkspaceRoot: root,
		Bash:          bash,
		Network:       in.Network,
		AllowWrite:    append([]string(nil), in.AllowWrite...),
		PreviewHosts:  normalizePreviewHosts(in.PreviewHosts),
		PreviewPorts:  normalizePreviewPorts(in.PreviewPorts),
		PreviewStrict: in.PreviewStrict,
	}
	if len(profile.PreviewHosts) == 0 {
		def := defaultProjectSandboxProfile(root)
		profile.PreviewHosts = def.PreviewHosts
	}
	if len(profile.PreviewPorts) == 0 {
		def := defaultProjectSandboxProfile(root)
		profile.PreviewPorts = def.PreviewPorts
	}
	return profile, nil
}

// probePreviewReachable checks TCP/HTTP from the native side (no WebView CORS).
func probePreviewReachable(raw string, timeout time.Duration) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if !strings.Contains(trimmed, "://") {
		parsed, err = url.Parse("http://" + trimmed)
	}
	if err != nil || parsed == nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return false
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return false
	}

	client := &http.Client{Timeout: timeout}
	target := parsed.String()

	req, err := http.NewRequest(http.MethodHead, target, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		resp, err = client.Get(target)
	}
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode < 600
}
