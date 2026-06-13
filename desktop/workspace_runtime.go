package main

import (
	"path/filepath"
	"strings"
	"sync"

	"arcdesk/internal/boot"
	"arcdesk/internal/config"
)

type workspaceRuntimeEntry struct {
	mu        sync.Mutex
	refs      int
	kit       *boot.WorkspaceKit
	preparing bool
	waiters   []chan struct{}
}

type workspaceRuntimePool struct {
	mu    sync.Mutex
	entries map[string]*workspaceRuntimeEntry
}

func newWorkspaceRuntimePool() *workspaceRuntimePool {
	return &workspaceRuntimePool{entries: map[string]*workspaceRuntimeEntry{}}
}

func workspaceRuntimeKey(scope, workspaceRoot string) string {
	root := strings.TrimSpace(workspaceRoot)
	if strings.TrimSpace(scope) == "global" || root == "" {
		root = globalTabWorkspaceRoot()
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return filepath.Clean(root)
}

func (p *workspaceRuntimePool) acquire(app *App, key string) (*boot.WorkspaceKit, error) {
	p.mu.Lock()
	ent, ok := p.entries[key]
	if !ok {
		ent = &workspaceRuntimeEntry{}
		p.entries[key] = ent
	}
	p.mu.Unlock()

	ent.mu.Lock()
	for ent.preparing {
		wait := make(chan struct{})
		ent.waiters = append(ent.waiters, wait)
		ent.mu.Unlock()
		<-wait
		ent.mu.Lock()
	}
	if ent.kit != nil {
		ent.refs++
		kit := ent.kit
		ent.mu.Unlock()
		return kit, nil
	}
	ent.preparing = true
	ent.mu.Unlock()

	kit, err := boot.PrepareWorkspaceKit(app.bootContext(), key, nil, true)
	ent.mu.Lock()
	ent.preparing = false
	if err != nil {
		for _, w := range ent.waiters {
			close(w)
		}
		ent.waiters = nil
		ent.mu.Unlock()
		return nil, err
	}
	ent.kit = kit
	ent.refs++
	for _, w := range ent.waiters {
		close(w)
	}
	ent.waiters = nil
	kitOut := ent.kit
	ent.mu.Unlock()
	return kitOut, nil
}

func (p *workspaceRuntimePool) release(key string) {
	p.mu.Lock()
	ent, ok := p.entries[key]
	if !ok {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	ent.mu.Lock()
	defer ent.mu.Unlock()
	if ent.refs > 0 {
		ent.refs--
	}
	if ent.refs == 0 && ent.kit != nil {
		ent.kit.Close()
		ent.kit = nil
		p.mu.Lock()
		if ent.refs == 0 {
			delete(p.entries, key)
		}
		p.mu.Unlock()
	}
}

func (p *workspaceRuntimePool) invalidate(key string) {
	p.mu.Lock()
	ent, ok := p.entries[key]
	if !ok {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	ent.mu.Lock()
	defer ent.mu.Unlock()
	if ent.kit != nil {
		ent.kit.Close()
		ent.kit = nil
	}
	config.InvalidateConfigCache(key)
}

func (a *App) workspaceKitForTab(tab *WorkspaceTab) (*boot.WorkspaceKit, string, error) {
	if a.wsRuntimes == nil {
		a.wsRuntimes = newWorkspaceRuntimePool()
	}
	key := workspaceRuntimeKey(tab.Scope, tab.WorkspaceRoot)
	kit, err := a.wsRuntimes.acquire(a, key)
	return kit, key, err
}

func (a *App) releaseWorkspaceKitForTab(tab *WorkspaceTab) {
	if a.wsRuntimes == nil || tab == nil {
		return
	}
	key := workspaceRuntimeKey(tab.Scope, tab.WorkspaceRoot)
	a.wsRuntimes.release(key)
}

func (a *App) invalidateWorkspaceKitForRoot(scope, workspaceRoot string) {
	if a.wsRuntimes == nil {
		return
	}
	key := workspaceRuntimeKey(scope, workspaceRoot)
	a.wsRuntimes.invalidate(key)
}
