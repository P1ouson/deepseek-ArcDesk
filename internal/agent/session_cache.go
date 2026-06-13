package agent

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

const sessionListCacheTTL = 3 * time.Second

type sessionListCacheEntry struct {
	mu        sync.Mutex
	infos     []SessionInfo
	cachedAt  time.Time
	dirStamp  int64 // latest mod time seen in dir (unix nano)
	fileCount int
}

var sessionListCache sync.Map // dir → *sessionListCacheEntry

func sessionDirStamp(dir string) (stamp int64, count int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, err
	}
	count = len(entries)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if fi, err := e.Info(); err == nil {
			if t := fi.ModTime().UnixNano(); t > stamp {
				stamp = t
			}
		}
	}
	return stamp, count, nil
}

// InvalidateSessionList drops the cached session index for dir. Pass "" to clear all.
func InvalidateSessionList(dir string) {
	if dir == "" {
		sessionListCache = sync.Map{}
		return
	}
	sessionListCache.Delete(filepath.Clean(dir))
}

// ListSessionsCached returns ListSessions results with a short TTL cache keyed on
// directory churn so boot and sidebar paths do not rescan large session folders.
func ListSessionsCached(dir string) ([]SessionInfo, error) {
	dir = filepath.Clean(dir)
	if dir == "" {
		return nil, nil
	}
	stamp, count, err := sessionDirStamp(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	now := time.Now()
	if v, ok := sessionListCache.Load(dir); ok {
		ent := v.(*sessionListCacheEntry)
		ent.mu.Lock()
		if ent.dirStamp == stamp && ent.fileCount == count && now.Sub(ent.cachedAt) < sessionListCacheTTL {
			out := append([]SessionInfo(nil), ent.infos...)
			ent.mu.Unlock()
			return out, nil
		}
		ent.mu.Unlock()
	}

	infos, err := ListSessions(dir)
	if err != nil {
		return nil, err
	}

	ent := &sessionListCacheEntry{
		infos:     append([]SessionInfo(nil), infos...),
		cachedAt:  now,
		dirStamp:  stamp,
		fileCount: count,
	}
	if v, loaded := sessionListCache.LoadOrStore(dir, ent); loaded {
		existing := v.(*sessionListCacheEntry)
		existing.mu.Lock()
		existing.infos = ent.infos
		existing.cachedAt = ent.cachedAt
		existing.dirStamp = ent.dirStamp
		existing.fileCount = ent.fileCount
		existing.mu.Unlock()
	}
	return infos, nil
}
