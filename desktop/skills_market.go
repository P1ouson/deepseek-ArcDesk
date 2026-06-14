package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"arcdesk/internal/hook"
	"arcdesk/internal/skill"
)

const skillsMarketSearchURL = "https://skills.sh/api/search"

// SkillsMarketEntry is one skill row from skills.sh search.
type SkillsMarketEntry struct {
	ID       string `json:"id"`
	SkillID  string `json:"skillId"`
	Name     string `json:"name"`
	Source   string `json:"source"`
	Installs int    `json:"installs"`
}

// SkillsMarketSearchResult is the frontend-facing skills.sh search payload.
type SkillsMarketSearchResult struct {
	Query   string              `json:"query"`
	Skills  []SkillsMarketEntry `json:"skills"`
	Count   int                 `json:"count"`
	HasMore bool                `json:"hasMore"`
	Page    int                 `json:"page"`
}

// InstallSkillsMarketResult reports where a marketplace skill was installed.
type InstallSkillsMarketResult struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
	Path  string `json:"path"`
}

type skillsMarketSearchResponse struct {
	Query  string              `json:"query"`
	Skills []SkillsMarketEntry `json:"skills"`
	Count  int                 `json:"count"`
	Error  string              `json:"error"`
}

var skillsMarketBrowseQueries = []string{
	"skill", "agent", "code", "design", "test", "deploy", "review", "api",
	"data", "cloud", "mobile", "web", "docs", "security", "marketing", "writing",
}

// SearchSkillsMarket queries skills.sh for installable skills.
// page is 0-based; later pages rotate through related browse queries so "load more"
// can surface additional catalog entries beyond the first batch.
func (a *App) SearchSkillsMarket(query string, limit int, page int) (SkillsMarketSearchResult, error) {
	if page < 0 {
		page = 0
	}
	if limit <= 0 {
		limit = 48
	}
	if limit > 100 {
		limit = 100
	}

	searchQuery := skillsMarketQueryForPage(query, page)
	if searchQuery == "" {
		return SkillsMarketSearchResult{
			Query:   strings.TrimSpace(query),
			Skills:  []SkillsMarketEntry{},
			Count:   0,
			HasMore: false,
			Page:    page,
		}, nil
	}

	u, err := url.Parse(skillsMarketSearchURL)
	if err != nil {
		return SkillsMarketSearchResult{}, err
	}
	q := u.Query()
	q.Set("q", searchQuery)
	q.Set("limit", fmt.Sprintf("%d", limit))
	u.RawQuery = q.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	body, err := fetchBytes(ctx, skillsMarketHTTPClient(), u.String())
	if err != nil {
		return SkillsMarketSearchResult{}, fmt.Errorf("skills.sh search: %w", err)
	}
	var resp skillsMarketSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return SkillsMarketSearchResult{}, fmt.Errorf("skills.sh search: invalid response")
	}
	if msg := strings.TrimSpace(resp.Error); msg != "" {
		return SkillsMarketSearchResult{}, fmt.Errorf("skills.sh: %s", msg)
	}
	if resp.Skills == nil {
		resp.Skills = []SkillsMarketEntry{}
	}
	return SkillsMarketSearchResult{
		Query:   searchQuery,
		Skills:  resp.Skills,
		Count:   resp.Count,
		HasMore: skillsMarketHasMore(query, page, len(resp.Skills), limit),
		Page:    page,
	}, nil
}

func skillsMarketQueryForPage(userQuery string, page int) string {
	userQuery = strings.TrimSpace(userQuery)
	if userQuery == "" {
		userQuery = "skill"
	}
	if page <= 0 {
		return userQuery
	}
	idx := page - 1
	if idx >= len(skillsMarketBrowseQueries) {
		return ""
	}
	candidate := skillsMarketBrowseQueries[idx]
	if strings.EqualFold(candidate, userQuery) {
		idx++
		if idx >= len(skillsMarketBrowseQueries) {
			return ""
		}
		candidate = skillsMarketBrowseQueries[idx]
	}
	return candidate
}

func skillsMarketHasMore(userQuery string, page, resultCount, limit int) bool {
	if skillsMarketQueryForPage(userQuery, page+1) == "" {
		return false
	}
	if resultCount >= limit {
		return true
	}
	return page+1 < len(skillsMarketBrowseQueries)
}

// SkillsMarketProjectInstallAvailable reports whether the active tab can receive project-scoped skills.
func (a *App) SkillsMarketProjectInstallAvailable() bool {
	a.mu.RLock()
	tab := a.activeTabLocked()
	a.mu.RUnlock()
	if tab == nil {
		return false
	}
	if tab.Scope != "project" {
		return false
	}
	root := strings.TrimSpace(tab.WorkspaceRoot)
	return root != "" && root != globalTabWorkspaceRoot()
}

// InstallSkillsMarketSkill downloads a skill from its GitHub source and installs it globally or into the active project.
func (a *App) InstallSkillsMarketSkill(source, skillID, scope string) (InstallSkillsMarketResult, error) {
	source = strings.TrimSpace(source)
	skillID = strings.TrimSpace(skillID)
	scope = strings.ToLower(strings.TrimSpace(scope))
	if source == "" || skillID == "" {
		return InstallSkillsMarketResult{}, fmt.Errorf("source and skillId are required")
	}
	if scope != "global" && scope != "project" {
		return InstallSkillsMarketResult{}, fmt.Errorf("scope must be global or project")
	}

	owner, repo, err := parseGitHubSource(source)
	if err != nil {
		return InstallSkillsMarketResult{}, err
	}

	projectRoot := ""
	if scope == "project" {
		if !a.SkillsMarketProjectInstallAvailable() {
			return InstallSkillsMarketResult{}, fmt.Errorf("open a project workspace tab to install skills into the project folder")
		}
		projectRoot = strings.TrimSpace(a.activeWorkspaceRoot())
		if projectRoot == "" {
			return InstallSkillsMarketResult{}, fmt.Errorf("no active project workspace")
		}
		if err := hook.Trust(projectRoot, ""); err != nil {
			return InstallSkillsMarketResult{}, fmt.Errorf("trust project: %w", err)
		}
	}

	repoPath, folderName, err := resolveSkillsMarketRepoPath(owner, repo, skillID, source+"/"+skillID)
	if err != nil {
		return InstallSkillsMarketResult{}, err
	}

	destRoot, err := skillsMarketDestRoot(scope, projectRoot)
	if err != nil {
		return InstallSkillsMarketResult{}, err
	}
	destDir := filepath.Join(destRoot, folderName)
	if _, err := os.Stat(destDir); err == nil {
		return InstallSkillsMarketResult{}, fmt.Errorf("skill %q already exists at %s", folderName, destDir)
	}

	if err := copyGitHubSkillDirectory(owner, repo, repoPath, destDir); err != nil {
		return InstallSkillsMarketResult{}, err
	}

	if err := a.RefreshSkills(); err != nil {
		return InstallSkillsMarketResult{}, err
	}
	return InstallSkillsMarketResult{
		Name:  folderName,
		Scope: scope,
		Path:  destDir,
	}, nil
}

func skillsMarketHTTPClient() *http.Client {
	return &http.Client{Timeout: 25 * time.Second}
}

func parseGitHubSource(source string) (owner, repo string, err error) {
	parts := strings.Split(strings.Trim(source, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid skill source %q", source)
	}
	return parts[0], parts[1], nil
}

func skillsMarketDestRoot(scope, projectRoot string) (string, error) {
	switch scope {
	case "global":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".arcdesk", skill.SkillsDirname), nil
	case "project":
		if strings.TrimSpace(projectRoot) == "" {
			return "", fmt.Errorf("project workspace is required")
		}
		return filepath.Join(projectRoot, ".arcdesk", skill.SkillsDirname), nil
	default:
		return "", fmt.Errorf("unsupported scope %q", scope)
	}
}

func skillRepoPathCandidates(source, skillID, fullID string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		p = strings.Trim(strings.TrimSpace(p), "/")
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	suffix := strings.TrimPrefix(strings.TrimSpace(fullID), strings.TrimSpace(source)+"/")
	if suffix == fullID {
		suffix = skillID
	}
	for _, token := range []string{skillID, suffix} {
		add("skills/" + token)
		add(token)
		replaced := strings.ReplaceAll(token, ":", "-")
		add("skills/" + replaced)
		add(replaced)
	}
	return out
}

func resolveSkillsMarketRepoPath(owner, repo, skillID, fullID string) (repoPath, folderName string, err error) {
	for _, candidate := range skillRepoPathCandidates(sourceFromOwnerRepo(owner, repo), skillID, fullID) {
		ok, name, probeErr := githubSkillDirExists(owner, repo, candidate)
		if probeErr != nil {
			err = probeErr
			continue
		}
		if ok {
			return candidate, name, nil
		}
	}
	found, findErr := findSkillDirInGitHubTree(owner, repo, skillID)
	if findErr != nil {
		if err != nil {
			return "", "", err
		}
		return "", "", findErr
	}
	return found, filepath.Base(found), nil
}

func sourceFromOwnerRepo(owner, repo string) string {
	return owner + "/" + repo
}

func githubSkillDirExists(owner, repo, repoPath string) (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, escapeGitHubPath(repoPath))
	body, err := fetchBytes(ctx, skillsMarketHTTPClient(), apiURL)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return false, "", nil
		}
		return false, "", err
	}
	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(body, &entries); err != nil {
		return false, "", err
	}
	hasSkill := false
	for _, e := range entries {
		if e.Type == "file" && strings.EqualFold(e.Name, skill.SkillFile) {
			hasSkill = true
			break
		}
	}
	if !hasSkill {
		return false, "", nil
	}
	return true, filepath.Base(repoPath), nil
}

func findSkillDirInGitHubTree(owner, repo, skillID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/main?recursive=1", owner, repo)
	body, err := fetchBytes(ctx, skillsMarketHTTPClient(), apiURL)
	if err != nil {
		return "", fmt.Errorf("locate skill in %s/%s: %w", owner, repo, err)
	}
	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(body, &tree); err != nil {
		return "", fmt.Errorf("parse GitHub tree: %w", err)
	}
	wantKeys := skillMatchKeys(skillID)
	var bestPath string
	bestScore := -1
	for _, item := range tree.Tree {
		if item.Type != "blob" || !strings.EqualFold(filepath.Base(item.Path), skill.SkillFile) {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(item.Path))
		base := filepath.Base(dir)
		score := scoreSkillDirMatch(base, wantKeys)
		if score > bestScore {
			bestScore = score
			bestPath = dir
		}
	}
	if bestPath == "" || bestScore < 0 {
		return "", fmt.Errorf("could not find %q in %s/%s on GitHub", skillID, owner, repo)
	}
	return bestPath, nil
}

func skillMatchKeys(skillID string) map[string]bool {
	keys := map[string]bool{}
	for _, token := range []string{
		skillID,
		strings.ReplaceAll(skillID, ":", "-"),
		strings.ReplaceAll(skillID, ":", ""),
		strings.ToLower(skillID),
		strings.ToLower(strings.ReplaceAll(skillID, ":", "-")),
	} {
		token = strings.TrimSpace(token)
		if token != "" {
			keys[token] = true
		}
	}
	return keys
}

func scoreSkillDirMatch(base string, want map[string]bool) int {
	base = strings.TrimSpace(base)
	if base == "" {
		return -1
	}
	if want[base] {
		return 100
	}
	lower := strings.ToLower(base)
	if want[lower] {
		return 90
	}
	for key := range want {
		if strings.EqualFold(base, key) {
			return 80
		}
	}
	return -1
}

func escapeGitHubPath(p string) string {
	parts := strings.Split(filepath.ToSlash(p), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func copyGitHubSkillDirectory(owner, repo, repoPath, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	return copyGitHubPath(owner, repo, repoPath, destDir)
}

func copyGitHubPath(owner, repo, repoPath, destDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, escapeGitHubPath(repoPath))
	body, err := fetchBytes(ctx, skillsMarketHTTPClient(), apiURL)
	if err != nil {
		return err
	}
	var entries []struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		Type        string `json:"type"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.Unmarshal(body, &entries); err != nil {
		return err
	}
	for _, entry := range entries {
		target := filepath.Join(destDir, entry.Name)
		switch entry.Type {
		case "dir":
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			if err := copyGitHubPath(owner, repo, entry.Path, target); err != nil {
				return err
			}
		case "file":
			if strings.TrimSpace(entry.DownloadURL) == "" {
				continue
			}
			data, err := fetchBytes(ctx, skillsMarketHTTPClient(), entry.DownloadURL)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(target, data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
