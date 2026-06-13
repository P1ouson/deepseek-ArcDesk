package gitrag

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// BlameLine is one blamed source line.
type BlameLine struct {
	Line    int    `json:"line"`
	Commit  string `json:"commit"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Summary string `json:"summary"`
	Text    string `json:"text"`
}

// Blame returns line-level blame for path. start/end are 1-based inclusive; 0 means all lines.
func (r *Repo) Blame(ctx context.Context, path string, start, end, limit int) ([]BlameLine, error) {
	if r == nil {
		return nil, fmt.Errorf("nil repo")
	}
	rel, err := r.relPath(path)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 200
	}
	args := []string{"blame", "--line-porcelain"}
	if start > 0 && end > 0 && end >= start {
		args = append(args, "-L", fmt.Sprintf("%d,%d", start, end))
	}
	args = append(args, "--", rel)
	out, err := runGit(ctx, r.Root, args...)
	if err != nil {
		return nil, err
	}
	lines := parsePorcelainBlame(out)
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return lines, nil
}

func parsePorcelainBlame(out string) []BlameLine {
	var res []BlameLine
	var cur BlameLine
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "\t") {
			cur.Text = strings.TrimPrefix(line, "\t")
			res = append(res, cur)
			cur = BlameLine{}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] != "author" && fields[0] != "author-mail" &&
			fields[0] != "author-time" && fields[0] != "summary" && fields[0] != "previous" {
			if n, err := strconv.Atoi(fields[2]); err == nil {
				cur.Line = n
				cur.Commit = fields[0]
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "author "):
			cur.Author = strings.TrimPrefix(line, "author ")
		case strings.HasPrefix(line, "author-time "):
			cur.Date = strings.TrimPrefix(line, "author-time ")
		case strings.HasPrefix(line, "summary "):
			cur.Summary = strings.TrimPrefix(line, "summary ")
		}
	}
	return res
}
