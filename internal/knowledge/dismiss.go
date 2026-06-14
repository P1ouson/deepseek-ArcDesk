package knowledge

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"arcdesk/internal/repomap"
)

const dismissedFile = "knowledge-dismissed.jsonl"

type dismissedRecord struct {
	Fingerprint string    `json:"fingerprint"`
	At          time.Time `json:"at"`
}

func dismissedPath(workspaceRoot string) (string, error) {
	dir, err := repomap.ProjectDir(workspaceRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, dismissedFile), nil
}

func loadDismissed(path string) (map[string]bool, error) {
	out := map[string]bool{}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec dismissedRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			fp := strings.TrimSpace(line)
			if fp != "" {
				out[fp] = true
			}
			continue
		}
		fp := strings.TrimSpace(rec.Fingerprint)
		if fp != "" {
			out[fp] = true
		}
	}
	return out, sc.Err()
}

// IsCaptureDismissed reports whether the user permanently ignored this fingerprint.
func IsCaptureDismissed(workspaceRoot, fingerprint string) bool {
	fingerprint = strings.TrimSpace(fingerprint)
	if workspaceRoot == "" || fingerprint == "" {
		return false
	}
	path, err := dismissedPath(workspaceRoot)
	if err != nil {
		return false
	}
	dismissed, err := loadDismissed(path)
	if err != nil {
		return false
	}
	return dismissed[fingerprint]
}

// DismissCapture permanently ignores a capture fingerprint for this workspace.
func DismissCapture(workspaceRoot, fingerprint string) error {
	fingerprint = strings.TrimSpace(fingerprint)
	if workspaceRoot == "" || fingerprint == "" {
		return nil
	}
	path, err := dismissedPath(workspaceRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	rec := dismissedRecord{Fingerprint: fingerprint, At: time.Now().UTC()}
	b, _ := json.Marshal(rec)
	_, err = f.Write(append(b, '\n'))
	return err
}
