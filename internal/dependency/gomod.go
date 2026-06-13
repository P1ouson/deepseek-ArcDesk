package dependency

import (
	"bufio"
	"os"
	"strings"
)

type goModFile struct {
	Module   string
	Requires []goModRequire
	Replaces []goModReplace
}

type goModRequire struct {
	Path    string
	Version string
}

type goModReplace struct {
	OldPath string
	NewPath string
}

func parseGoMod(path string) (*goModFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info := &goModFile{}
	sc := bufio.NewScanner(f)
	inBlock := ""
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "module ") {
			info.Module = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			continue
		}
		switch line {
		case "require (":
			inBlock = "require"
			continue
		case "replace (":
			inBlock = "replace"
			continue
		case ")":
			inBlock = ""
			continue
		}
		switch inBlock {
		case "require":
			if req := parseRequireLine(line); req.Path != "" {
				info.Requires = append(info.Requires, req)
			}
		case "replace":
			if rep := parseReplaceLine(line); rep.OldPath != "" {
				info.Replaces = append(info.Replaces, rep)
			}
		default:
			if strings.HasPrefix(line, "require ") {
				if req := parseRequireLine(strings.TrimPrefix(line, "require ")); req.Path != "" {
					info.Requires = append(info.Requires, req)
				}
			} else if strings.HasPrefix(line, "replace ") {
				if rep := parseReplaceLine(strings.TrimPrefix(line, "replace ")); rep.OldPath != "" {
					info.Replaces = append(info.Replaces, rep)
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return info, nil
}

func parseRequireLine(line string) goModRequire {
	line = strings.TrimSpace(line)
	if line == "" {
		return goModRequire{}
	}
	if i := strings.Index(line, "//"); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return goModRequire{}
	}
	req := goModRequire{Path: fields[0]}
	if len(fields) > 1 {
		req.Version = fields[1]
	}
	return req
}

func parseReplaceLine(line string) goModReplace {
	line = strings.TrimSpace(line)
	if i := strings.Index(line, "//"); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	const arrow = " => "
	if idx := strings.Index(line, arrow); idx >= 0 {
		return goModReplace{
			OldPath: strings.TrimSpace(line[:idx]),
			NewPath: strings.TrimSpace(line[idx+len(arrow):]),
		}
	}
	return goModReplace{}
}
