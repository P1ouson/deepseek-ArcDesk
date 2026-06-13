package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	terminalOutputEvent = "terminal:output"
	terminalExitEvent   = "terminal:exit"
)

// terminalSession wraps a PTY-backed interactive shell.
type terminalSession struct {
	id     string
	pty    ptyCloser
	waitFn func() int
	closed chan struct{}
}

type ptyCloser interface {
	io.ReadWriteCloser
	resize(cols, rows uint16) error
}

type terminalManager struct {
	mu       sync.Mutex
	sessions map[string]*terminalSession
	nextSeq  int
}

func newTerminalManager() *terminalManager {
	return &terminalManager{sessions: map[string]*terminalSession{}}
}

func (m *terminalManager) closeSessionLocked(s *terminalSession) {
	if s == nil {
		return
	}
	close(s.closed)
	_ = s.pty.Close()
	delete(m.sessions, s.id)
}

func (m *terminalManager) Close(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sessionID == "" {
		ids := make([]string, 0, len(m.sessions))
		for id := range m.sessions {
			ids = append(ids, id)
		}
		for _, id := range ids {
			m.closeSessionLocked(m.sessions[id])
		}
		return
	}
	m.closeSessionLocked(m.sessions[sessionID])
}

func (m *terminalManager) CloseAll() {
	m.Close("")
}

func (m *terminalManager) Start(app *App, cwd string) (id, shell, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	name, args := terminalShellArgs(app)
	cmd := exec.Command(name, args...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	pty, waitFn, err := startPTY(cmd)
	if err != nil {
		return "", "", err.Error()
	}

	m.nextSeq++
	id = fmt.Sprintf("term-%d", m.nextSeq)
	s := &terminalSession{id: id, pty: pty, waitFn: waitFn, closed: make(chan struct{})}
	m.sessions[id] = s

	go m.readLoop(app, s)
	go m.waitLoop(app, s)

	return id, name, ""
}

func (m *terminalManager) readLoop(app *App, s *terminalSession) {
	buf := make([]byte, 4096)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			emitTerminalOutput(app, s.id, buf[:n])
		}
		if err != nil {
			return
		}
		select {
		case <-s.closed:
			return
		default:
		}
	}
}

func (m *terminalManager) waitLoop(app *App, s *terminalSession) {
	code := 0
	if s.waitFn != nil {
		code = s.waitFn()
	}
	emitTerminalExit(app, s.id, code)
	m.mu.Lock()
	if cur, ok := m.sessions[s.id]; ok && cur == s {
		m.closeSessionLocked(s)
	}
	m.mu.Unlock()
}

func (m *terminalManager) Write(sessionID, dataB64 string) error {
	m.mu.Lock()
	s := m.sessions[sessionID]
	m.mu.Unlock()
	if s == nil {
		return io.EOF
	}
	raw, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	_, err = s.pty.Write(raw)
	return err
}

func (m *terminalManager) Resize(sessionID string, cols, rows int) error {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	m.mu.Lock()
	s := m.sessions[sessionID]
	m.mu.Unlock()
	if s == nil {
		return nil
	}
	return s.pty.resize(uint16(cols), uint16(rows))
}

func emitTerminalOutput(app *App, sessionID string, data []byte) {
	if app == nil || app.ctx == nil || len(data) == 0 || sessionID == "" {
		return
	}
	app.ingestTerminalOutput(sessionID, data)
	runtime.EventsEmit(app.ctx, terminalOutputEvent, map[string]string{
		"id":   sessionID,
		"data": base64.StdEncoding.EncodeToString(data),
	})
}

func emitTerminalExit(app *App, sessionID string, code int) {
	if app == nil || app.ctx == nil || sessionID == "" {
		return
	}
	runtime.EventsEmit(app.ctx, terminalExitEvent, map[string]any{
		"id":   sessionID,
		"code": code,
	})
}

func terminalShellArgs(app *App) (string, []string) {
	pref := ""
	if app != nil {
		if cfg, _, err := app.loadDesktopUserConfigForEdit(); err == nil && cfg != nil {
			pref = cfg.DesktopTerminalShell()
		}
	}
	if goruntime.GOOS == "windows" {
		return terminalShellWindows(pref)
	}
	if pref == "wsl" {
		if p, err := exec.LookPath("wsl.exe"); err == nil {
			return p, nil
		}
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	if p, err := exec.LookPath(shell); err == nil {
		shell = p
	}
	return shell, nil
}

func terminalShellWindows(pref string) (string, []string) {
	switch pref {
	case "cmd":
		if p, err := exec.LookPath("cmd.exe"); err == nil {
			return p, nil
		}
		return "cmd.exe", nil
	case "git-bash":
		candidates := []string{
			filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe"),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Git", "bin", "bash.exe"),
		}
		for _, candidate := range candidates {
			if candidate != "" {
				if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
					return candidate, []string{"--login", "-i"}
				}
			}
		}
		if p, err := exec.LookPath("bash.exe"); err == nil {
			return p, []string{"--login", "-i"}
		}
	case "wsl":
		if p, err := exec.LookPath("wsl.exe"); err == nil {
			return p, nil
		}
		return "wsl.exe", nil
	}
	for _, name := range []string{"pwsh.exe", "pwsh", "powershell.exe", "powershell"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, []string{"-NoLogo"}
		}
	}
	return "powershell.exe", []string{"-NoLogo"}
}
