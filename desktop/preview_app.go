package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var devPreviewPorts = []int{5173, 5174, 3000, 3001, 8080, 4173, 8000}

// DetectDevServerURL scans common local dev ports and returns the first reachable preview URL.
func (a *App) DetectDevServerURL() string {
	for _, port := range devPreviewPorts {
		for _, host := range []string{"127.0.0.1", "localhost"} {
			candidate := "http://" + host + ":" + strconv.Itoa(port)
			if probePreviewReachable(candidate, 1200*time.Millisecond) {
				return candidate
			}
		}
	}
	return ""
}

// OpenWebPreview opens the in-app Web preview panel (emits app:open-web-preview).
func (a *App) OpenWebPreview(raw string) {
	url := strings.TrimSpace(raw)
	if url == "" {
		url = a.DetectDevServerURL()
	}
	if url == "" {
		url = "http://localhost:5173"
	}
	v := a.ValidatePreviewURL(url)
	if v.Decision == "blocked" || strings.TrimSpace(v.URL) == "" {
		return
	}
	runtime.EventsEmit(a.ctx, "app:open-web-preview", map[string]any{"url": v.URL})
}
