package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var previewPageExtensions = map[string]bool{
	".html": true, ".htm": true, ".xhtml": true, ".svg": true,
}

func isPreviewPageExt(path string) bool {
	return previewPageExtensions[strings.ToLower(filepath.Ext(path))]
}

type pagePreviewServer struct {
	mu    sync.Mutex
	ln    net.Listener
	port  int
	bases map[string]string // token -> absolute workspace root
}

func newPagePreviewServer() *pagePreviewServer {
	return &pagePreviewServer{bases: map[string]string{}}
}

func (s *pagePreviewServer) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		_ = s.ln.Close()
		s.ln = nil
	}
}

func tokenForBase(absRoot string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(absRoot)))
	return hex.EncodeToString(sum[:8])
}

func (s *pagePreviewServer) ensure() error {
	s.mu.Lock()
	if s.ln != nil {
		s.mu.Unlock()
		return nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		s.mu.Unlock()
		return err
	}
	s.ln = ln
	s.port = ln.Addr().(*net.TCPAddr).Port
	s.mu.Unlock()
	go http.Serve(ln, http.HandlerFunc(s.serveHTTP))
	return nil
}

func (s *pagePreviewServer) registerBase(absRoot string) (token string, err error) {
	absRoot = filepath.Clean(absRoot)
	if absRoot == "" {
		return "", fmt.Errorf("empty workspace root")
	}
	if err := s.ensure(); err != nil {
		return "", err
	}
	token = tokenForBase(absRoot)
	s.mu.Lock()
	s.bases[token] = absRoot
	s.mu.Unlock()
	return token, nil
}

func (s *pagePreviewServer) previewURL(absRoot, rel string) (string, error) {
	token, err := s.registerBase(absRoot)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(strings.TrimPrefix(filepath.Clean(rel), `\`))
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" || rel == "." {
		rel = "index.html"
	}
	s.mu.Lock()
	port := s.port
	s.mu.Unlock()
	return fmt.Sprintf("http://127.0.0.1:%d/p/%s/%s", port, token, rel), nil
}

func (s *pagePreviewServer) baseForToken(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	root, ok := s.bases[token]
	return root, ok
}

func (s *pagePreviewServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	const prefix = "/p/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.NotFound(w, r)
		return
	}
	token, rel := parts[0], parts[1]
	base, ok := s.baseForToken(token)
	if !ok {
		http.NotFound(w, r)
		return
	}
	rel = filepath.FromSlash(rel)
	full, ok, err := workspacePathForBase(base, rel)
	if err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.IsDir() {
		for _, name := range []string{"index.html", "index.htm"} {
			candidate := filepath.Join(full, name)
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				full = candidate
				info = st
				break
			}
		}
		if info.IsDir() {
			http.NotFound(w, r)
			return
		}
	}
	ext := mime.TypeByExtension(strings.ToLower(filepath.Ext(full)))
	if ext == "" {
		ext = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ext)
	w.Header().Set("Cache-Control", "no-cache")
	if r.Method == http.MethodHead {
		return
	}
	f, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	_, _ = io.Copy(w, f)
}

// WorkspacePagePreviewURL serves a workspace HTML/SVG page over loopback and returns its URL.
func (a *App) WorkspacePagePreviewURL(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("path is required")
	}
	path, ok, err := a.workspacePath(rel)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", os.ErrInvalid
	}
	if !isPreviewPageExt(path) {
		return "", fmt.Errorf("not a previewable page")
	}
	base, err := a.activeWorkspaceBase()
	if err != nil {
		return "", err
	}
	if a.pagePreview == nil {
		a.pagePreview = newPagePreviewServer()
	}
	url, err := a.pagePreview.previewURL(base, rel)
	if err != nil {
		slog.Warn("workspace page preview", "err", err)
		return "", err
	}
	return url, nil
}

// IsPreviewablePage reports whether rel is an HTML/SVG page the preview panel can render.
func (a *App) IsPreviewablePage(rel string) bool {
	path, ok, err := a.workspacePath(strings.TrimSpace(rel))
	if err != nil || !ok {
		return false
	}
	return isPreviewPageExt(path)
}
