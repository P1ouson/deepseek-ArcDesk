package main

import (
	goruntime "runtime"
	"os"

	"arcdesk/internal/hook"
)

// migrateSensitiveDataFileModes tightens permissions on secret-bearing files written
// before the 9C hardening batch (which used 0644).
func migrateSensitiveDataFileModes() {
	for _, name := range []string{
		"claw-channels.json",
		"mobile-connect.json",
		"mobile-sessions.json",
	} {
		ensurePrivateFileMode(ARCDESKDesktopDataPath(name))
	}
	hook.EnsureTrustFilePrivate("")
}

func ensurePrivateFileMode(path string) {
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err != nil {
		return
	}
	if goruntime.GOOS == "windows" {
		return
	}
	_ = os.Chmod(path, 0o600)
}
