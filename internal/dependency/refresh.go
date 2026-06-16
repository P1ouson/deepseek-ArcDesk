package dependency

import (
	"context"
	"errors"
	"sync"
	"time"

	"arcdesk/internal/reporeuse"
)

var refreshLocks sync.Map

func refreshLockFor(root string) *sync.Mutex {
	v, _ := refreshLocks.LoadOrStore(root, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// RefreshIfStale rebuilds the index when missing, forced stale, or fingerprint/git changed.
// It holds the write lock for the entire rebuild (<5s target); readers block until done.
func (i *Index) RefreshIfStale(ctx context.Context) error {
	if i == nil {
		return errors.New("nil index")
	}
	mu := refreshLockFor(i.root)
	if !mu.TryLock() {
		return nil
	}
	defer mu.Unlock()

	i.mu.Lock()
	defer i.mu.Unlock()

	if i.graph != nil && i.meta != nil && !i.forceStale && !CheckStale(i.root, i.meta) {
		return nil
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	if i.graph != nil && i.meta != nil && !i.forceStale {
		newHead := gitHead(i.root)
		newFP := ComputeFingerprint(i.root)
		if reporeuse.HeadChangedFingerprintStable(i.meta.GitHead, newHead, i.meta.Fingerprint, newFP) {
			paths, err := reporeuse.ChangedFilesBetween(i.root, i.meta.GitHead, newHead)
			if err == nil && !reporeuse.PathsAffectDependency(paths) {
				return i.bumpGitHeadLocked()
			}
		}
	}

	g, meta, err := BuildGraph(BuildOptions{Root: i.root})
	if err != nil {
		return err
	}
	dir, err := ProjectDir(i.root)
	if err != nil {
		return err
	}
	if err := SaveIndex(g, dir); err != nil {
		return err
	}
	if err := SaveMeta(meta, dir); err != nil {
		return err
	}
	i.graph = g
	i.meta = meta
	i.forceStale = false
	return nil
}

// InvalidateFiles marks the index stale; Phase 1 triggers full rebuild on next refresh.
func (i *Index) InvalidateFiles(paths []string) error {
	if i == nil {
		return errors.New("nil index")
	}
	_ = paths
	i.mu.Lock()
	i.forceStale = true
	i.mu.Unlock()
	return nil
}

func (i *Index) bumpGitHeadLocked() error {
	head := gitHead(i.root)
	if i.meta == nil {
		i.meta = &Meta{IndexVersion: IndexVersion}
	}
	i.meta.GitHead = head
	i.meta.GeneratedAt = time.Now().UTC()
	i.meta.Fingerprint = ComputeFingerprint(i.root)
	dir, err := ProjectDir(i.root)
	if err != nil {
		return err
	}
	if err := SaveMeta(i.meta, dir); err != nil {
		return err
	}
	i.forceStale = false
	return nil
}
