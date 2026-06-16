package repomap

import (
	"path/filepath"
	"strings"
	"sync"
)

type revisionEntry struct {
	gitHead     string
	fingerprint string
}

var (
	revisionMu    sync.RWMutex
	revisionCache = map[string]revisionEntry{}
)

// InvalidateWorkspaceRevision drops the cached HEAD/fingerprint for a workspace.
// Call after successful writes so plancache keys and provenance stay fresh.
func InvalidateWorkspaceRevision(workspace string) {
	key, ok := revisionCacheKey(workspace)
	if !ok {
		return
	}
	revisionMu.Lock()
	delete(revisionCache, key)
	revisionMu.Unlock()
}

func revisionCacheKey(workspace string) (string, bool) {
	abs, err := filepath.Abs(strings.TrimSpace(workspace))
	if err != nil || abs == "" {
		return "", false
	}
	return filepath.Clean(abs), true
}

func cachedWorkspaceRevision(workspace string) (gitHead, fingerprint string, ok bool) {
	key, valid := revisionCacheKey(workspace)
	if !valid {
		return "", "", false
	}
	revisionMu.RLock()
	entry, ok := revisionCache[key]
	revisionMu.RUnlock()
	if !ok {
		return "", "", false
	}
	return entry.gitHead, entry.fingerprint, true
}

func storeWorkspaceRevision(workspace, gitHead, fingerprint string) {
	key, ok := revisionCacheKey(workspace)
	if !ok {
		return
	}
	revisionMu.Lock()
	revisionCache[key] = revisionEntry{gitHead: gitHead, fingerprint: fingerprint}
	revisionMu.Unlock()
}
